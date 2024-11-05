exports.isActive = function() {
  return $.domain == "404media.co"
}

exports.processMeta = function(){
  $.authors = [ $.meta['twitter.data1']  ]
}
