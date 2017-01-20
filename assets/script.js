function updateGateways() {
  window.fetch('/gateways')
    .then(function(response) {
      return response.text()
    })
    .then(function(response) {
      return JSON.parse(response)
    })
    .then(function(gateways) {
      document.getElementById("gateways-list").innerHTML = gateways.join(", ")
    });
}

var socket = io()
var events = document.getElementById("events")
socket.on('status', function (msg) {
  var message = JSON.parse(msg)
  var html = (
    '<div class="event status">' +
      '<div class="time">' + new Date().toLocaleString() + '</div>' +
      '<div class="title">status</div>' +
      '<div class="data">' +
        JSON.stringify(message, null, 2) +
      '</div>' +
    '</div>'
  )
  events.insertAdjacentHTML('afterbegin', html)
})
socket.on('gtw-connect', function (gatewayID) {
  var html = (
    '<div class="event">' +
      '<div class="time">' + new Date().toLocaleString() + '</div>' +
      '<div class="title">connect</div>' +
      '<div class="data">' +
        gatewayID +
      '</div>' +
    '</div>'
  )
  events.insertAdjacentHTML('afterbegin', html)
  updateGateways()
})
socket.on('gtw-disconnect', function (gatewayID) {
  var html = (
    '<div class="event">' +
      '<div class="time">' + new Date().toLocaleString() + '</div>' +
      '<div class="title">disconnect</div>' +
      '<div class="data">' +
        gatewayID +
      '</div>' +
    '</div>'
  )
  events.insertAdjacentHTML('afterbegin', html)
  updateGateways()
})
socket.on('uplink', function (msg) {
  var message = JSON.parse(msg)
  delete message.Message.trace
  var html = (
    '<div class="event uplink">' +
      '<div class="time">' + new Date().toLocaleString() + '</div>' +
      '<div class="title">uplink</div>' +
      '<div class="data">' +
        JSON.stringify(message, null, 2) +
      '</div>' +
    '</div>'
  )
  events.insertAdjacentHTML('afterbegin', html)
})
socket.on('downlink', function (msg) {
  var message = JSON.parse(msg)
  var trace = message.Message.trace.parents.map(function (el) {
    el.time = new Date(el.time / 1000000)
    return el
  })
  delete message.Message.trace
  var html = (
    '<div class="event downlink">' +
      '<div class="time">' + new Date().toLocaleString() + '</div>' +
      '<div class="title">downlink</div>' +
      '<div class="data">' +
        JSON.stringify(message, null, 2) +
      '</div>' +
    '</div>'
  )
  events.insertAdjacentHTML('afterbegin', html)
  visualizeEvents("#trace", trace)
})

updateGateways()
