import { Controller } from "@hotwired/stimulus"

export default class extends Controller {
  static get values() {
    return {
      action: String,
      method: String,
    }
  }

  connect() {
    let tagName = this.element.tagName.toLowerCase()
    switch (tagName) {
    case "form":
      this.element.setAttribute("data-turbo", "true")
      this.conditionnalAttr("action", this.actionValue, this.hasActionValue)
      this.conditionnalAttr("method", this.methodValue, this.hasMethodValue)
      break
    case "button":
      this.element.closest("form").setAttribute("data-turbo", "true")
      this.conditionnalAttr("formaction", this.actionValue, this.hasActionValue)
      this.conditionnalAttr("formmethod", this.methodValue, this.hasMethodValue)
      break
    default:
      throw new Error("turbo-form can only be used on form and button elements")
    }
  }

  conditionnalAttr(name, value, condition) {
    if (condition) {
      this.element.setAttribute(name, value)
    }
  }

  stopSubmit(evt) {
    evt.stopPropagation()
  }
}
