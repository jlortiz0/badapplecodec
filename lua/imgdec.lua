local expect = require("cc.expect").expect

local function decoderReadBit(d)
  if d.bytePos == 8 then
    d.curByte = d.data.read(1)
    if d.curByte == nil then
      return nil
    end
    d.curByte = string.byte(d.curByte)
    d.bytePos = 0
  end
  b = bit32.band(d.curByte, bit32.lshift(1, d.bytePos))
  d.bytePos = d.bytePos + 1
  return b ~= 0 
end

local function decoderBeginRLE(d)
  pos = 1
  b = decoderReadBit(d)
  while b do
    pos = pos + 1
    b = decoderReadBit(d)
  end
  if b == nil then return end
  add = 0
  for i=0, pos-1  do
    add = bit32.lshift(add, 1)
    b = decoderReadBit(d)
    if b then add = add + 1 end
  end
  d.packetLen = bit32.lshift(1, pos) - 1 + add
end

local function decoderReadHeader(d, bits)
  expect(1, d, "table")
  expect(2, bits, "number", "nil")
  d.lastBit = false
  bits = bits or 0
  b = decoderReadBit(d)
  header = 0
  for i=0, bits-1 do
    if b == nil then return nil end
    if b then header = bit32.bor(header, bit32.lshift(1, i)) end
    b = decoderReadBit(d)
  end
  if b == nil then return nil end
  if not b then decoderBeginRLE(d) end
  return header
end

local function decoderDestroy(d)
  d.data.close()
end

local function decoderReadCrumb(d)
  if d.packetLen > 0 then
    d.packetLen = d.packetLen - 1
    if d.lastBit then return 3 else return 0 end
  end
  b1 = decoderReadBit(d)
  b2 = decoderReadBit(d)
  if b1 == nil or b2 == nil then return nil end
  if not (b1 or b2) then
    decoderBeginRLE(d)
    return decoderReadCrumb(d)
  end
  out = 0
  if b1 then d.lastBit = not d.lastBit end
  if d.lastBit then out = 2 end
  if b2 then d.lastBit = not d.lastBit end
  if d.lastBit then out = out + 1 end
  return out
end

local function NewRLEDecoder(f)
  local d = {curByte = 0, data = f, packetLen = 0, bytePos = 8, lastBit = false}
  return {
    readHeader = function(bits) return decoderReadHeader(d, bits) end,
    readCrumb = function() return decoderReadCrumb(d) end,
    destroy = function() decoderDestroy(d) end,
    _read = f.read
  }
end

local function idecDestroy(d) d.dec.destroy() end  

local function idecGetSize(d)
  return d.w, d.h
end

local function xorTab(t1, t2)
  for i,v in ipairs(t2) do
    t1[i] = bit32.bxor(t1[i], v)
  end
  return t1
end

local function nibCombine(t1, t2)
  local out = {}
  for i,v in ipairs(t1) do
    local d1, d2 = 0, 0
    if bit32.band(v, 2) ~= 0 then d1 = 2 end
    if bit32.band(v, 1) ~= 0 then d2 = 2 end
    v = t2[i]
    if bit32.band(v, 2) ~= 0 then d1 = d1 + 1 end
    if bit32.band(v, 1) ~= 0 then d2 = d2 + 1 end
    out[2 * i - 1] = d1
    out[2 * i] = d2
  end
  return out
end

local ccColors = { colors.black, colors.gray, colors.lightGray, colors.white }

local function idecRead(d)
  local len = d.h * d.w
  len = len / 2
  header1 = d.dec.readHeader(2)
  if header1 == nil then return nil end
  plane1 = {}
  for i = 1, len do
    plane1[i] = d.dec.readCrumb()
    if plane1[i] == nil then return nil end
  end
  header2 = d.dec.readHeader(2)
  if header2 == nil then return nil end
  plane2 = {}
  for i = 1, len do
    plane2[i] = d.dec.readCrumb()
    if plane2[i] == nil then return nil end
  end
  if bit32.band(header2, 1) ~= 0 then plane2 = xorTab(plane2, plane1) end
  if bit32.band(header1, 1) ~= 0 then plane1, plane2 = plane2, plane1 end
  plane = nibCombine(plane1, plane2)
  if bit32.band(header1, 2) ~= 0 then plane = xorTab(plane, d.lastPlane) end
  d.lastPlane = plane
  temp = {}
  for i,v in ipairs(plane) do temp[i] = ccColors[v + 1] end
  return temp
end

local function NewImageDecoder(fname)
  local f = fs.open(fname, "rb")
  if f == nil then error("could not open file") end
  if f.read(4) ~= "JBAC" then error("invalid format") end
  local h = 256 * string.byte(f.read(1)) + string.byte(f.read(1))
  local w = 256 * string.byte(f.read(1)) + string.byte(f.read(1))
  local lastPlane = {}
  for i=0, h*w do lastPlane[i] = 0 end
  local d = {
    h = h, w = w, lastPlane = lastPlane, dec = NewRLEDecoder(f)
  }
  return {
    destroy = function() idecDestroy(d) end,
    getSize = function() return idecGetSize(d) end,
    read = function() return idecRead(d) end
  }
end

 return { newDecoder = NewImageDecoder, read = idecRead, getSize = idecGetSize, destroy = idecDestroy }
-- return { NewRLEDecoder = NewRLEDecoder, destroy = decoderDestroy, readHeader = decoderReadHeader, readCrumb = decoderReadCrumb }
