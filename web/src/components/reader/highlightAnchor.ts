export interface HighlightAnchor {
  layer: 'original' | 'translation';
  paragraph_index: number;
  text_start_offset: number;
  text_end_offset: number;
  quoted_text: string;
}

export function computeAnchor(range: Range): HighlightAnchor | null {
  if (range.collapsed) return null;

  const startParagraph = findParagraphElement(range.startContainer);
  const endParagraph = findParagraphElement(range.endContainer);
  if (!startParagraph || startParagraph !== endParagraph) return null;

  const indexStr = startParagraph.getAttribute('data-paragraph-index');
  if (indexStr == null) return null;

  const layer =
    (startParagraph.closest('[data-layer]')?.getAttribute('data-layer') as
      | 'original'
      | 'translation'
      | null) ?? 'original';

  const quotedText = range.toString();
  if (!quotedText.trim()) return null;

  const textStartOffset = getTextOffset(startParagraph, range.startContainer, range.startOffset);
  const textEndOffset = getTextOffset(startParagraph, range.endContainer, range.endOffset);

  if (textStartOffset >= textEndOffset) return null;

  return {
    layer,
    paragraph_index: Number.parseInt(indexStr, 10),
    text_start_offset: textStartOffset,
    text_end_offset: textEndOffset,
    quoted_text: quotedText,
  };
}

function findParagraphElement(node: Node): HTMLElement | null {
  let current: Node | null = node;
  while (current) {
    if (current instanceof HTMLElement && current.hasAttribute('data-paragraph-index')) {
      return current;
    }
    current = current.parentElement;
  }
  return null;
}

function getTextOffset(root: Node, targetNode: Node, targetOffset: number): number {
  let offset = 0;
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  let node: Node | null;

  while ((node = walker.nextNode())) {
    if (node === targetNode) {
      return offset + targetOffset;
    }
    offset += node.textContent?.length ?? 0;
  }

  return offset + targetOffset;
}
