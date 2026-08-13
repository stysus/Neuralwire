/**
 * A lightweight, safe Markdown to HTML parser.
 * Supports:
 * - Fenced code blocks (```code```)
 * - Inline code (`code`)
 * - Headers (# H1, ## H2, ### H3)
 * - Bold (**text**) and Italics (*text*)
 * - Lists (- item or * item)
 * - Paragraphs (separated by double newlines)
 */
export function markdownToHtml(md: string): string {
	if (!md) return '';

	// Normalize newlines
	let html = md.replace(/\r\n/g, '\n');

	// 1. Fenced code blocks
	const codeBlockRegex = /```(\w*)\n([\s\S]*?)```/g;
	html = html.replace(codeBlockRegex, (_, lang, code) => {
		const escapedCode = escapeHtml(code.trim());
		const langClass = lang ? ` class="language-${lang}"` : '';
		return `<pre><code${langClass}>${escapedCode}</code></pre>`;
	});

	// Split by block type to avoid parsing markdown inside code blocks
	const blocks = html.split(/(<pre>[\s\S]*?<\/pre>)/g);

	for (let i = 0; i < blocks.length; i++) {
		// Only process non-pre blocks
		if (!blocks[i].startsWith('<pre>')) {
			let block = blocks[i];

			// 2. Headers
			block = block.replace(/^### (.*?)$/gm, '<h3>$1</h3>');
			block = block.replace(/^## (.*?)$/gm, '<h2>$1</h2>');
			block = block.replace(/^# (.*?)$/gm, '<h1>$1</h1>');

			// 3. Unordered Lists
			// We group consecutive list items
			const listRegex = /^([*-])\s+(.*?)$/gm;
			block = block.replace(listRegex, '<li>$2</li>');
			// Wrap adjacent list items in <ul>
			block = block.replace(/(<li>[\s\S]*?<\/li>)/g, '<ul>$1</ul>');
			// Merge adjacent <ul> tags
			block = block.replace(/<\/ul>\s*<ul>/g, '');

			// 4. Bold and Italics
			block = block.replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>');
			block = block.replace(/\*(.*?)\*/g, '<em>$1</em>');
			block = block.replace(/__(.*?)__/g, '<strong>$1</strong>');
			block = block.replace(/_(.*?)_/g, '<em>$1</em>');

			// 5. Inline code
			block = block.replace(/`(.*?)`/g, '<code>$1</code>');

			// 6. Paragraphs (double newlines)
			// Split by double newlines, wrap in <p> if not already block elements
			const lines = block.split(/\n\n+/);
			const processedLines = lines.map(line => {
				const trimmed = line.trim();
				if (!trimmed) return '';
				// If it already starts with a block tag, don't wrap it
				if (/^(<h[1-6]|<ul|<ol|<li|<blockquote|<p)/i.test(trimmed)) {
					return trimmed;
				}
				return `<p>${trimmed.replace(/\n/g, '<br />')}</p>`;
			});
			block = processedLines.filter(Boolean).join('\n');

			blocks[i] = block;
		}
	}

	return blocks.join('\n');
}

/**
 * Escapes special HTML characters to prevent XSS and rendering issues inside code blocks.
 */
function escapeHtml(text: string): string {
	return text
		.replace(/&/g, '&amp;')
		.replace(/</g, '&lt;')
		.replace(/>/g, '&gt;')
		.replace(/"/g, '&quot;')
		.replace(/'/g, '&#039;');
}

/**
 * A utility to check if string contains HTML tags.
 */
export function isHtml(text: string): boolean {
	return /<[a-z][\s\S]*>/i.test(text);
}
