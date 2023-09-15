// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

const colors = require("tailwindcss/colors")

// source:
// https://uicolors.app/edit?sv1=golden-tainoi:50-fff9eb/100-ffefc6/200-ffdd88/300-ffcb5c/400-ffae20/500-f98a07/600-dd6402/700-b74406/800-94330c/900-7a2c0d/950-461402;mandy:50-fef2f3/100-fee5e7/200-fccfd6/300-f9a8b3/400-f5778a/500-ed4e6b/600-d9254f/700-b71942/800-99183d/900-83183a/950-49081b;pale-oyster:50-fdfcfc/100-f8f7f6/200-ded9d3/300-c0b4aa/400-a7978b/500-968273/600-81695f/700-69544f/800-594946/900-4c3f3d/950-291f1f;shakespeare:50-f2f9fd/100-e5f1f9/200-c4e4f3/300-91cee8/400-43acd6/500-309bc7/600-217da8/700-1c6488/800-1b5571/900-1b475f/950-122e3f
const palette = {
  blue: {
    50: "#F2FBFC",
    100: "#E3F8FA",
    200: "#BDEAF0",
    300: "#97DBE6",
    400: "#55C1D4",
    500: "#1ba1bf",
    600: "#178DAD",
    700: "#0E6A8F",
    800: "#094E73",
    900: "#053657",
    950: "#021F38",
  },
  orange: {
    50: "#fff9eb",
    100: "#ffefc6",
    200: "#ffdd88",
    300: "#ffcb5c",
    400: "#ffae20",
    500: "#f98a07",
    600: "#dd6402",
    700: "#b74406",
    800: "#94330c",
    900: "#7a2c0d",
    950: "#461402",
  },
  red: {
    50: "#fef4f6",
    100: "#fce2e7",
    200: "#f8bdc8",
    300: "#f498a9",
    400: "#f0738a",
    500: "#ed4e6b",
    600: "#e9294c",
    700: "#d31638",
    800: "#a0112b",
    900: "#6d0b1d",
    950: "#540916",
  },
  gray: {
    50: "#fdfcfc",
    100: "#f8f7f6",
    200: "#ded9d3",
    300: "#c0b4aa",
    400: "#a7978b",
    500: "#968273",
    600: "#81695f",
    700: "#69544f",
    800: "#594946",
    900: "#4c3f3d",
    950: "#291f1f",
  },
}

// prettier-ignore
var config = {
  content: [
    "src/**/*.js",
    "../assets/templates/**/*.jet.html",
  ],
  theme: {
    colors: {
      transparent: "transparent",
      current: "currentColor",
      black: colors.black,
      white: colors.white,
      gray: palette.gray,
      red: palette.red,
      green: colors.lime,
      blue: palette.blue,
      yellow: palette.orange,
      primary: {
        light: palette.blue[300],
        DEFAULT: palette.blue[600],
        dark: palette.blue[800],
      },
    },
    fontFamily: {
      sans: [
        "public sans", "sans-serif",
        "Apple Color Emoji", "Segoe UI Emoji",
        "Segoe UI Symbol", "Noto Color Emoji",
      ],
      serif: [
        "lora", "serif",
        "Apple Color Emoji", "Segoe UI Emoji",
        "Segoe UI Symbol", "Noto Color Emoji",
      ],
      mono: [
        "ui-monospace", "Cascadia Code", "Source Code Pro", "Menlo", "Consolas", "DejaVu Sans Mono", "monospace",
        "Apple Color Emoji", "Segoe UI Emoji",
        "Segoe UI Symbol", "Noto Color Emoji",
      ],
    },
    screens: {
      "xs": {raw: "only screen and (min-width: 340px)"},
      "sm": {raw: "only screen and (min-width: 640px)"},
      "md": {raw: "only screen and (min-width: 814px)"},
      "lg": {raw: "only screen and (min-width: 1024px)"},
      "xl": {raw: "only screen and (min-width: 1280px)"},
      "max-xl": {raw: "only screen and (max-width: 1280px)"},
      "max-lg": {raw: "only screen and (max-width: 1024px)"},
      "max-md": {raw: "only screen and (max-width: 814px)"},
      "max-sm": {raw: "only screen and (max-width: 640px)"},
      "max-xs": {raw: "only screen and (max-width: 340px)"},
    },
    extend: {
      boxShadow: {
        "sidebar-l": "5px 0 10px -5px rgba(0, 0, 0, 0.1)",
        "panel-t": "0 4px 6px 6px rgba(0, 0, 0, 0.1)",
      },
      contrast: {
        "105": "1.05",
      },
      fontSize: {
        "h1": "2.5rem",
        "h2": "2rem",
        "h3": "1.5rem",
      },
      gridTemplateColumns: {
        "bk-tools": "2fr auto auto",
        "cards": "repeat(auto-fill, minmax(12rem, 1fr))",
      },
      height: {
        "screen": "100vh",
        "max-content": "max-content",
        "topnav": "4rem",
      },
      padding: {
        "16/9": "56.25%",
      },
      spacing: {
        "0.5": "0.125rem",
        "topnav": "4rem",
        ...[...Array(26).keys()].reduce((acc, x) => {
          acc[`col-${x+1}`] = `${(x+1) * 3.5}rem`
          return acc
        }, {}),
      },
      width: {
      },
      maxWidth: {
        ...[...Array(26).keys()].reduce((acc, x) => {
          acc[`col-${x+1}`] = `${(x+1) * 3.5}rem`
          return acc
        }, {}),
      },
    },
  },
  plugins: [
    require("./ui/plugins/interactions"),
    require("./ui/plugins/forms"),
  ],
}

module.exports = config
