if (!String.prototype.padStart) {
  String.prototype.padStart = function (max, fillString) {
    return padStart(this, max, fillString);
  };
}

function padStart (text, max, mask) {
  var cur = text.length;
  if (max <= cur) {
    return text;
  }
  var masked = max - cur;
  var filler = String(mask) || ' ';
  while (filler.length < masked) {
    filler += filler;
  }
  var fillerSlice = filler.slice(0, masked);
  return fillerSlice + text;
}

if (!String.prototype.padEnd) {
  String.prototype.padEnd = function (max, fillString) {
    return padEnd(this, max, fillString);
  };
}

function padEnd (text, max, mask) {
  var cur = text.length;
  if (max <= cur) {
    return text;
  }
  var masked = max - cur;
  var filler = String(mask) || ' ';
  while (filler.length < masked) {
    filler += filler;
  }
  var fillerSlice = filler.slice(0, masked);
  return text + fillerSlice;
}
