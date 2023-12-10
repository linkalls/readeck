// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

exports.isActive = function () {
  return $.domain == "youtube.com"
}

exports.setConfig = function (config) {
  // there's no need for custom headers, on the contrary
  config.httpHeaders = {}
}

exports.processMeta = function () {
  if ($.meta["schema.identifier"].length == 0) {
    return
  }

  const videoID = $.meta["schema.identifier"][0]

  const info = getVideoInfo(videoID)

  // Get more information
  const lengthSeconds = info.videoDetails?.lengthSeconds
  if (lengthSeconds) {
    $.meta["x.duration"] = lengthSeconds
  }

  // Get transcript
  const transcript = getTranscript(info)
  if (transcript) {
    $.html = `<section id="main"><p>${transcript.join("<br>\n")}</p></section>`
    // we must force readability here for it to pick up the content
    // (it normally won't with a video)
    $.readability = true
  }
}

function getVideoInfo(videoID) {
  let rsp = requests.post(
    "https://youtubei.googleapis.com/youtubei/v1/player",
    JSON.stringify({
      context: {
        client: {
          hl: "en",
          clientName: "WEB",
          clientVersion: "2.20210721.00.00",
          mainAppWebInfo: {
            graftUrl: "/watch?v=" + videoID,
          },
        },
      },
      videoId: videoID,
    }),
    {
      "Content-Type": "application/json",
    },
  )
  rsp.raiseForStatus()
  return rsp.json()
}

function getTranscript(info) {
  const langPriority = ["en", undefined, null, ""]

  // Fetch caption list
  let captions =
    info.captions?.playerCaptionsTracklistRenderer?.captionTracks || []
  captions = captions.map((x) => {
    x.auto = x.kind == "asr"
    return x
  })

  // Look for a default track
  let trackIdx =
    info.captions?.playerCaptionsTracklistRenderer?.audioTracks?.find(
      (x) => x.hasDefaultTrack,
    )?.defaultCaptionTrackIndex

  let track
  if (trackIdx !== null) {
    // If we have a default track, we take this one.
    track = captions[trackIdx]
  } else {
    // If we don't have a caption index, we sort the list by automatic
    // caption last and language code priorities.
    captions.sort((a, b) => {
      return (
        a.auto - b.auto ||
        langPriority.indexOf(b.languageCode) -
          langPriority.indexOf(a.languageCode)
      )
    })

    track = (captions || []).find(() => true)
  }

  if (!track) {
    return
  }

  console.debug("found transcript", { track })

  const rsp = requests.get(track.baseUrl)
  rsp.raiseForStatus()

  return (decodeXML(rsp.text()).transcript?.text || [])
    .map((x) => {
      return x["#text"]
    })
    .filter((x) => x)
}
