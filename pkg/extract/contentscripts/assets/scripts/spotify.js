// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

exports.isActive = function () {
  return $.domain == "spotify.com"
}

exports.setConfig = function (config) {
  config.httpHeaders["User-Agent"] = "curl/7.0"
}
