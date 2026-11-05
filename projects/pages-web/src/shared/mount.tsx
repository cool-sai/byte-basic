import { StrictMode, type ReactNode } from "react"
import { createRoot } from "react-dom/client"
import "./styles.css"

/** 每个页面入口调用一次：mount(<ListPage />) */
export function mount(node: ReactNode) {
  createRoot(document.getElementById("root")!).render(<StrictMode>{node}</StrictMode>)
}
