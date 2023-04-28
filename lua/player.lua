local decoder = require("imgdec")

local function mainThread(fname)
  local d = decoder.newDecoder(fname)
  local w, h = d.getSize()
  local mw, mh = term.getSize()
  if mw < w or mh < h then
      print("Minimum dimensions: "..w.."x"..h)
      print("Found dimensions: "..mw.."x"..mh)
      d.destroy()
      return
  end
  local data, bgColor = d.read()
  local almID = os.startTimer(0.05)
  local startW = math.floor((mw - w) / 2) + 1
  local startH = math.floor((mh - h) / 2) + 1
  local wpos = 1
  local hpos = startH
  while data ~= nil do
      term.setBackgroundColor(bgColor)
      term.setCursorPos(startW,startH)
      local lastBg = bgColor
      bgRun = ""
      for _,v in ipairs(data) do
          if v == lastBg then
             bgRun = bgRun .. " "
          else
              if #bgRun > 0 then
                write(bgRun)
              end
              term.setBackgroundColor(v)
              lastBg = v
              bgRun = " "
          end
          wpos = wpos + 1
          if wpos > w then
              if #bgRun > 0 then
                write(bgRun)
                bgRun = ""
              end
              term.setBackgroundColor(bgColor)
              lastBg = bgColor
              hpos = hpos + 1
              term.setCursorPos(startW, hpos)
              wpos = 1
          end
      end
      wpos = 1
      hpos = startH
      data = d.read()
      local alId = almID - 1
      while alId ~= almID do
          _, alId = os.pullEvent("timer")
      end
      almID = os.startTimer(0.05)
  end
  d.destroy()
end

if #{...} == 0 then
    print("Usage: player <.bac file>")
    return
end

term.clear()
while true do
mainThread(shell.resolve(...))
end

