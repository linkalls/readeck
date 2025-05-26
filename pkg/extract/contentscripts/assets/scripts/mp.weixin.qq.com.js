// SPDX-FileCopyrightText: © 2025 linsui
//
// SPDX-License-Identifier: AGPL-3.0-only

exports.priority = 10;

exports.isActive = () => $.host === "mp.weixin.qq.com";

exports.setConfig = (config) => {
  config.replaceStrings.push(["visibility: hidden", "visibility: visible"]);
};
