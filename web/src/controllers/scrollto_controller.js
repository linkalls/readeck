import {Controller} from "@hotwired/stimulus"

export default class extends Controller {
  /**
   *
   * @param {Event} event
   * @returns
   */
  scroll(event) {
    if (event.target.tagName.toLowerCase() != "a") {
      return
    }
    let url
    try {
      url = new URL(event.target.href)
    } catch (e) {
      console.error(e)
      return
    }

    if (!url.hash) {
      return
    }

    const el = document.querySelector(url.hash)
    if (!el) {
      return
    }

    el.scrollIntoView({block: "start", inline: "nearest", behavior: "auto"})
  }
}
