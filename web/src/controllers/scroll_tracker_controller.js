// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"

export default class extends Controller {
  static classes = ["down"]
  static values = {
    threshold: { type: Number, default: 100 } // Minimum scroll difference to trigger show on upscroll
  }

  connect() {
    let prevScroll = window.scrollY
    let isDown = false
    let ticking = false
    window.addEventListener(
      "scroll",
      () => {
        if (!ticking) {
          window.requestAnimationFrame(() => {
            ticking = false
            const currentScroll = window.scrollY
            const scrollDiff = currentScroll - prevScroll
            
            if (scrollDiff > 0) {
              // Immediately hide on downscroll
              isDown = true
              this.element.classList.add(this.downClass)
              prevScroll = currentScroll
            } else if (scrollDiff < 0 && Math.abs(scrollDiff) >= this.thresholdValue) {
              // Show on upscroll only if threshold is met
              isDown = false
              this.element.classList.remove(this.downClass)
              prevScroll = currentScroll
            }
          })

          ticking = true
        }
      },
      {passive: true},
    )
  }
}
