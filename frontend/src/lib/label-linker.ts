import { useEffect } from "react"

// Many forms put a <Label> next to its control without htmlFor/id. This ties
// each such label to the first control that follows it in the same group,
// so screen readers announce the field's name and clicking the label
// focuses the field. It also covers forms added later.

const CONTROL = "input:not([type=hidden]),textarea,select,button[role=combobox],button[role=switch],[data-slot=input-otp] input"
let seq = 0

function link(root: ParentNode) {
  root.querySelectorAll<HTMLLabelElement>("label[data-slot=label]:not([for])").forEach((label) => {
    if (label.querySelector(CONTROL) || !label.parentElement) return // wraps its control already
    let control: Element | null = null
    for (let el = label.nextElementSibling; el && !control; el = el.nextElementSibling) {
      control = el.matches(CONTROL) ? el : el.querySelector(CONTROL)
    }
    if (!control || control.hasAttribute("aria-label") || control.hasAttribute("aria-labelledby")) return
    if (!control.id) control.id = `field-${++seq}`
    label.htmlFor = control.id
  })
}

/** Keeps labels linked as screens and dialogs render. */
export function useLabelLinker() {
  useEffect(() => {
    link(document)
    let pending = false
    const obs = new MutationObserver(() => {
      if (pending) return
      pending = true
      requestAnimationFrame(() => {
        pending = false
        link(document)
      })
    })
    obs.observe(document.body, { childList: true, subtree: true })
    return () => obs.disconnect()
  }, [])
}
