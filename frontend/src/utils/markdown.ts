import DOMPurify from 'dompurify'
import MarkdownIt from 'markdown-it'

const md = new MarkdownIt({
  html: false,
  breaks: true,
  linkify: true,
  typographer: false
})

/** Renders model Markdown into safe HTML for chat messages. */
export function renderMarkdown(text: string): string {
  return DOMPurify.sanitize(md.render(text || ''))
}
