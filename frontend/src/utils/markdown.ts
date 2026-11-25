import DOMPurify from 'dompurify'
import MarkdownIt from 'markdown-it'

const md = new MarkdownIt({
  html: false,
  breaks: false,
  linkify: true,
  typographer: false
})

/**
 * Normalises model output before rendering. Single newlines inside a paragraph
 * are kept as soft breaks, while block elements (headings and lists) get a
 * blank line so Markdown-It produces real paragraphs with proper spacing.
 */
export function formatAssistantAnswer(text: string): string {
  if (!text) return ''

  const lines = text.replace(/\r\n?/g, '\n').trim().split('\n')
  const out: string[] = []
  let inFence = false

  for (const raw of lines) {
    const line = raw.replace(/[ \t]+$/, '')
    if (line.trim().startsWith('```')) inFence = !inFence

    const isBlock = !inFence && (/^\s*([-*+] |\d+\. )/.test(line) || /^#{1,6} /.test(line))
    if (isBlock && out.length > 0 && out[out.length - 1].trim() !== '') {
      out.push('')
    }
    out.push(line)
  }

  return out.join('\n').replace(/\n{3,}/g, '\n\n')
}

/** Renders model Markdown into safe HTML for chat messages and evaluation results. */
export function renderMarkdown(text: string): string {
  return DOMPurify.sanitize(md.render(formatAssistantAnswer(text)))
}
