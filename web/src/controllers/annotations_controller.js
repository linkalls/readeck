// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

import {Controller} from "@hotwired/stimulus"
import {request} from "../lib/request"

export default class extends Controller {
  static targets = ["root", "controlls", "controllCreate", "controllDelete"]
  static classes = ["hidden"]
  static values = {
    apiUrl: String,
    canCreate: {type: Boolean, default: false},
    canDelete: {type: Boolean, default: false},
  }

  connect() {
    // Listen for new selections
    this.annotation = null
    document.addEventListener("selectionchange", async (evt) => {
      await this.onSelectText(evt)
    })

    // Prepare controll box
    this.controllArrow = this.setupControll(
      "ontouchstart" in document.documentElement ? "bottom" : "top",
    )
  }

  canCreateValueChanged(value) {
    if (value) {
      this.controllCreateTarget.classList.remove(this.hiddenClass)
      this.setControllArrowColor(
        getComputedStyle(this.controllCreateTarget).getPropertyValue(
          "background-color",
        ),
      )
    } else {
      this.controllCreateTarget.classList.add(this.hiddenClass)
    }
  }

  canDeleteValueChanged(value) {
    if (value) {
      this.controllDeleteTarget.classList.remove(this.hiddenClass)
      this.setControllArrowColor(
        getComputedStyle(this.controllDeleteTarget).getPropertyValue(
          "background-color",
        ),
      )
    } else {
      this.controllDeleteTarget.classList.add(this.hiddenClass)
    }
  }

  async onSelectText() {
    // We must wait for next tick so it won't trigger when the event triggers
    // from a click on an existing selection.
    await this.nextTick()

    this.annotation = new Annotation(this.rootTarget, document.getSelection())
    if (this.annotation.isValid()) {
      await this.showControlls(true, false)
    } else if (this.annotation.coveredAnnotations().length > 0) {
      await this.showControlls(false, true)
    } else {
      await this.hideControlls()
    }
  }

  /**
   *
   * @param {string} position
   * @returns {Element}
   */
  setupControll(position) {
    if (!this.hasControllsTarget) {
      return
    }

    const arrow = document.createElement("div")
    const bt = "8px solid transparent"
    const bg = "8px solid var(--arrow-color)"

    arrow.dataset.position = position
    arrow.style.setProperty("--arrow-color", "rgba(0,0,0,0)")
    arrow.style.height = 0
    arrow.style.width = 0
    arrow.style.borderLeft = bt
    arrow.style.borderRight = bt
    if (position == "top") {
      arrow.style.borderTop = bg
      this.controllsTarget.appendChild(arrow)
    } else {
      arrow.style.borderBottom = bg
      this.controllsTarget.insertBefore(arrow, this.controllsTarget.firstChild)
    }

    return arrow
  }

  setControllArrowColor(color) {
    if (!this.controllArrow) {
      return
    }
    this.controllArrow.style.setProperty("--arrow-color", color)
  }

  /**
   *
   * @param {Boolean} canCreate
   * @param {Boolean} canDelete
   */
  async showControlls(canCreate, canDelete) {
    this.canCreateValue = canCreate
    this.canDeleteValue = canDelete
    await this.nextTick()

    const position = this.controllArrow.dataset.position

    // Show controlls
    this.controllsTarget.classList.remove(this.hiddenClass)

    // Get root, range and controlls coordinates
    const rangeRect = this.annotation.range.getBoundingClientRect()
    const rootRect = this.findRelativeRoot().getBoundingClientRect()

    // Controlls dimension
    const h = this.controllsTarget.clientHeight
    const w = this.controllsTarget.clientWidth

    // Range position relative to its root element
    const rangeTop =
      position == "top"
        ? Math.round(rangeRect.top - rootRect.top)
        : Math.round(rangeRect.top + rangeRect.height - rootRect.top)
    const rangeLeft = Math.round(rangeRect.left - rootRect.left)
    const rangeCenter = Math.round(rangeLeft + rangeRect.width / 2)

    // Set controlls position
    const y = position == "top" ? Math.round(rangeTop - h) : rangeTop
    // prettier-ignore
    const x = Math.floor(
      Math.max(
        0,
        Math.min(
          rangeCenter - w / 2,
          rootRect.width - w - 1,
        ),
      ),
    )

    this.controllsTarget.style.top = `${position == "top" ? y - 4 : y + 4}px`
    this.controllsTarget.style.left = `${x}px`

    // Set arrow position
    if (!this.controllArrow) {
      return
    }
    const arrowWidth = this.controllArrow.offsetWidth
    // prettier-ignore
    const arrowX = Math.max(
      arrowWidth / 2,
      Math.min(
        rangeCenter - x - arrowWidth / 2,
        w - arrowWidth - arrowWidth / 2,
      ),
    )
    this.controllArrow.style.marginLeft = `${arrowX}px`
  }

  async hideControlls() {
    this.canCreateValue = false
    this.canDeleteValue = false
    this.controllsTarget.classList.add(this.hiddenClass)
  }

  async nextTick() {
    return await new Promise((resolve) => setTimeout(resolve, 0))
  }

  findRelativeRoot() {
    let p = this.rootTarget
    while (p.parentElement) {
      if (getComputedStyle(p).position == "relative") {
        return p
      }
      p = p.parentElement
    }
    return p
  }

  /**
   * reload loads and replace the turbo frame content
   */
  reload = async () => {
    if (!this.element.src) {
      throw new Error("controller element must have an src attribute")
    }

    // Enable turbo frame and wait for it to be reloaded
    this.element.disabled = false
    await this.element.loaded
  }

  /**
   * save creates a new annotation on the document
   */
  async save(event) {
    const color = event.currentTarget.dataset.annotationsColorValue
    this.annotation.color = color

    if (!this.annotation || !this.annotation.isValid()) {
      return
    }

    await request(this.apiUrlValue, {
      method: "POST",
      body: {
        start_selector: this.annotation.startSelector,
        start_offset: this.annotation.startOffset,
        end_selector: this.annotation.endSelector,
        end_offset: this.annotation.endOffset,
        color: this.annotation.color,
      },
    })
    await this.reload()
  }

  async delete() {
    const ids = new Set()
    this.annotation.coveredAnnotations().forEach((n) => {
      ids.add(n.dataset.annotationIdValue)
    })

    if (ids.length == 0) {
      return
    }

    const baseURL = new URL(`${this.apiUrlValue}/`, document.URL)
    for (const id of ids) {
      const url = new URL(id, baseURL)
      await request(url, {method: "DELETE"})
    }

    await this.reload()
  }
}

/**
 * @callback walkTextNodesCallback
 * @param {Node} node
 * @param {int} index
 */

class Annotation {
  /**
   * Annotation holds raw information about an annotation. It contains only text with
   * selectors and offsets providing needed information to find an annotation.
   *
   * @param {Node} root
   * @param {Selection} selection
   */
  constructor(root, selection) {
    /** @type {Node} */
    this.root = root

    /** @type {Selection} */
    this.selection = selection

    /** @type {Range} */
    this.range = null

    /** @type {Node[]} */
    this.textNodes = []

    /** @type {Node} */
    this.ancestor = null

    /** @type {string} */
    this.startSelector = null

    /** @type {int} */
    this.startOffset = null

    /** @type {string} */
    this.endSelector = null

    /** @type {int} */
    this.endOffset = null

    /** @type {string} */
    this.color = null

    this.init()
  }

  init() {
    // Selection must be a range and contains something
    if (
      this.selection.type.toLowerCase() != "range" ||
      !this.selection.toString().trim()
    ) {
      return
    }

    // Only one range
    if (this.selection.rangeCount != 1) {
      return
    }

    const range = this.selection.getRangeAt(0)
    if (range.collapsed) {
      return
    }

    // Range must be within root element boundaries
    if (!this.root.contains(range.commonAncestorContainer)) {
      return
    }

    // This handles double click on an element (in opposition to selecting text).
    // Containers can be element and we only want to deal with text nodes.
    if (range.startContainer.nodeType == Node.ELEMENT_NODE) {
      walkTextNodes(range.startContainer, (n, i) => {
        if (i == 0) {
          range.setStart(n, 0)
        }
      })
    }
    if (range.endContainer.nodeType == Node.ELEMENT_NODE) {
      let c = range.endContainer
      if (range.endOffset == 0) {
        c = range.endContainer.previousElementSibling
      }
      walkTextNodes(c, (n) => {
        range.setEnd(n, n.textContent.length)
      })
    }

    // start and end containers must be text nodes
    if (
      range.startContainer.nodeType != Node.TEXT_NODE ||
      range.endContainer.nodeType != Node.TEXT_NODE
    ) {
      return
    }

    this.range = range
    const s = getSelector(
      this.root,
      this.range.startContainer,
      this.range.startOffset,
    )
    const e = getSelector(
      this.root,
      this.range.endContainer,
      this.range.endOffset,
    )
    this.startSelector = s.selector
    this.startOffset = s.offset
    this.endSelector = e.selector
    this.endOffset = e.offset

    // Get ancestor
    if (this.range.commonAncestorContainer.nodeType == Node.TEXT_NODE) {
      this.ancestor = this.range.commonAncestorContainer.parentElement
    } else {
      this.ancestor = this.range.commonAncestorContainer
    }

    // Collect included text nodes
    let started = false
    let done = false
    walkTextNodes(this.ancestor, (n) => {
      if (done) {
        return
      }
      if (n == this.range.startContainer) {
        started = true
      }
      if (started) {
        this.textNodes.push(n)
      }
      if (n == this.range.endContainer) {
        done = true
      }
    })
  }

  /**
   * @returns {Node[]}
   */
  coveredAnnotations() {
    return this.textNodes
      .filter((n) => n.parentElement.tagName.toLowerCase() == "rd-annotation")
      .map((n) => n.parentElement)
  }

  /**
   * @returns {Boolean}
   */
  isValid() {
    return this.range != null && this.coveredAnnotations().length == 0
  }
}

/**
 * getSelector returns a CSS selector for a text node at the given offset.
 *
 * @param {Node} root
 * @param {Node} node
 * @param {int} offset
 * @returns {{selector: string, offset: int}}
 */
function getSelector(root, node, offset) {
  let p = node.parentElement
  const names = []

  // Get selector
  while (p.parentElement && p != root) {
    let i = 1
    let s = p
    while (s.previousElementSibling) {
      s = s.previousElementSibling
      if (s.tagName.toLowerCase() == p.tagName.toLowerCase()) {
        i++
      }
    }
    names.unshift(`${p.tagName.toLowerCase()}[${i}]`)

    p = p.parentElement
  }

  // Get offset
  let done = false
  let newOffset = 0
  walkTextNodes(node.parentElement, (n, i) => {
    if (done) {
      return
    }
    if (n == node) {
      done = true
    }
    if (!done) {
      newOffset += n.textContent.length
    } else {
      newOffset += offset
    }
  })

  return {selector: names.join("/"), offset: newOffset}
}

/**
 * walkTextNodes calls a callback function on each text node
 * found in a given node and its descendants.
 *
 * @param {Node} node
 * @param {walkTextNodesCallback} callback
 * @param {int} [index]
 */
function walkTextNodes(node, callback, index) {
  index = index === undefined ? 0 : index
  for (let child = node.firstChild; child; child = child.nextSibling) {
    if (child.nodeType == Node.TEXT_NODE) {
      callback(child, index)
      index++
    }
    walkTextNodes(child, callback, index)
  }
}
