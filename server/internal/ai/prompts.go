package ai

import "fmt"

func TitleTranslationPrompt(targetLang string) string {
	switch targetLang {
	case "zh-CN":
		return "你是一个翻译助手。将以下标题翻译成中文，只输出翻译结果，不要任何解释。"
	case "en":
		return "You are a translation assistant. Translate the following title into English. Output only the translation, no explanations."
	default:
		return fmt.Sprintf("Translate the following title into %s. Output only the translation.", targetLang)
	}
}

func SummaryPrompt(targetLang string) string {
	switch targetLang {
	case "zh-CN":
		return "你是一个信息摘要助手。请用中文将以下文章浓缩为一段200字以内的摘要，语言简洁流畅，不要分点，不要使用列表符号，只输出摘要正文。"
	case "en":
		return "You are a summarization assistant. Summarize the following article in one concise paragraph of no more than 100 words. Do not use bullet points or lists. Output only the summary paragraph."
	default:
		return fmt.Sprintf("Summarize the following article in one concise paragraph of no more than 100 words in %s. No bullet points or lists. Output only the summary.", targetLang)
	}
}

func CombinedTitleSummaryPrompt(targetLang string) string {
	switch targetLang {
	case "zh-CN":
		return `你是一个翻译和摘要助手。请完成以下两个任务：
1. 将标题翻译成中文
2. 将正文浓缩为一段200字以内的中文摘要，语言简洁流畅，不要分点

严格按以下格式输出，不要输出其他任何内容：
TITLE: 翻译后的标题
SUMMARY: 摘要正文`
	case "en":
		return `You are a translation and summarization assistant. Complete these two tasks:
1. Translate the title into English
2. Summarize the body in one concise paragraph of no more than 100 words

Output strictly in this format, nothing else:
TITLE: translated title
SUMMARY: summary paragraph`
	default:
		return fmt.Sprintf(`Translate the title and summarize the body in %s.

Output strictly in this format, nothing else:
TITLE: translated title
SUMMARY: summary paragraph (max 100 words, no bullet points)`, targetLang)
	}
}

func CombinedTitleSummaryUserMessage(title, content string) string {
	const maxChars = 8000
	if len(content) > maxChars {
		content = content[:maxChars]
	}
	return fmt.Sprintf("标题: %s\n\n正文: %s", title, content)
}
