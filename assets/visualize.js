var offsets = {
  "bridge": 1,
  "router": 2,
  "broker": 3,
  "networkserver": 4,
  "handler": 5,
}

var colors = {
  "bridge": "rgb(0, 51, 153)",
  "router": "rgb(204, 102, 0)",
  "broker": "rgb(51, 153, 51)",
  "networkserver": "rgb(255, 204, 0)",
  "handler": "rgb(102, 0, 153)",
}

var dotSize = 10
var padding = 2
var axisHeight = 20

var height = (dotSize + padding) * 6 + axisHeight

var tooltip = d3.select("body").append("div")
  .attr("class", "tooltip")
  .style("opacity", 0)

function fmtTime(d) {
  minutes = ("" + d.getMinutes()).padStart(2, "0")
  seconds = ("" + d.getSeconds()).padStart(2, "0")
  millis = ("" + d.getMilliseconds()).padEnd(3, "0")
  return d.getHours() + ":" + minutes + ":" + seconds + "." + millis
}

function visualizeEvents(selector, evts) {
  var svg = d3.select(selector)
    .style("height", height + "px")

  var width = svg.node().clientWidth
  if (width === 0) {
    width = svg.node().parentNode.clientWidth
  }

  var xScale = d3.scaleTime()
    .domain([
      d3.min(evts, function (el) { return el.time }),
      d3.max(evts, function (el) { return el.time }),
    ])
    .range([dotSize, width - dotSize])

  var axis = svg.select("line")
    .attr("x2", width + "px")

  var circles = svg.selectAll("circle").data(evts)

  circles
    .text(function (d) { return d.service_name })
    .style("fill", function (d) {
      if (colors[d.service_name]) {
        return colors[d.service_name]
      }
      return "black"
    })
    .transition().duration(500).ease(d3.easeBounce)
    .delay(function (d, i) { return i * 10 })
    .attr("cy", function (d) { return (dotSize + padding) * offsets[d.service_name] + "px" })
    .attr("cx", function (d) { return xScale(d.time) + "px" })

  var currentEvent = d3.select("#current-event")

  circles.enter().append("circle")
    .on("mouseover", function (d) {
      d3.select(this).transition().duration(50).attr("r", function (d) { return dotSize + "px" })
      var meta = ""
      if (d.metadata) {
        var metaList = []
        Object.keys(d.metadata).forEach(function(k) {
          metaList.push("<code>" + k + "=" + d.metadata[k] + "</code>")
        })
        meta = " (" + metaList.join(", ") + ")"
      }
      currentEvent.html(d.event + " at " + d.service_name + ' <i>"' + d.service_id + '"</i> ' + meta)
    })
    .on("mouseout", function (d) {
      d3.select(this).transition().duration(50).attr("r", function (d) { return dotSize / 2 + "px" })
      currentEvent.text("")
    })
    .text(function (d) { return d.service_name })
    .attr("r", function (d) { return dotSize / 2 + "px" })
    .attr("cy", "-10px")
    .attr("cx", function (d) { return xScale(d.time) + "px" })
    .style("fill", function (d) {
      if (colors[d.service_name]) {
        return colors[d.service_name]
      }
      return "black"
    })
    .transition().duration(500).ease(d3.easeBounce)
    .delay(function (d, i) { return i * 10 })
    .attr("cy", function (d) { return (dotSize + padding) * offsets[d.service_name] + "px" })

  circles.exit()
    .transition().duration(100).ease(d3.easeCubicOut)
      .attr("cx", function (d) { return (width + dotSize/2 + 100) + "px" })
    .transition().delay(100).remove()

  var axis = d3.axisBottom(xScale)
    .tickSizeOuter(0)
    .tickArguments([d3.timeMillisecond.every(100)])
    .tickFormat(fmtTime)

  svg.select("g")
    .attr("transform", "translate(0," + (height - axisHeight) + ")")
    .call(axis)
}

visualizeEvents("#trace", [])
