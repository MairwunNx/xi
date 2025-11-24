package artificial

import (
	"fmt"
	"time"
)

const (
	EnvironmentBlockTemplate = `

⸻

📅 Environment
	• Date & time: %s (UTC+3)
	• Chat title: %s
	• Chat description: %s
	• Bot version: %s
	• Bot uptime: %s`

	UserRequestTemplate = `System data:
Date and time: %s
Participant: '%s'

Message:
%s`

	PersonalizationBlockTemplate = `

⸻

🙋‍♂️ Personalization

This is user-provided information about themselves. Consider this data when relevant and reasonable:

%s`
)

// formatUserRequest wraps a user message with system context (timestamp and persona)
func formatUserRequest(persona string, req string) string {
	localNow := time.Now().In(time.Local)
	timestamp := localNow.Format("Monday, 02 January 2006, 15:04:05")

	return fmt.Sprintf(UserRequestTemplate, timestamp, persona, req)
}
func getDefaultContextSelectionPrompt() string {
	return `You are a context selection agent. Your task is to analyze a conversation history and select which messages are relevant for answering the new user question.

Conversation history:
%s

New user question:
%s

Goal:
Select all messages that help the model correctly understand and respond to the new message — including implied or indirect context.

Evaluate relevance using these rules (in order of importance):

1. **Direct reference** — If the new message refers to, quotes, or builds upon earlier content (even indirectly), include that part fully.
2. **Logical and conversational flow** — If several messages form a continuous conversation (even with slight topic drift), include the whole chain until the topic clearly changes.
3. **Semantic relation** — Include messages that are conceptually or emotionally related, even if they use different words or topics.  
   *Example: “А какие вопросы?” → previous Q&A context.*
4. **Recency and tone** — Prefer the most recent messages, especially those establishing tone, personality, or current topic focus.
5. **Continuity bias** — When uncertain, **err on the side of including more context**, especially recent relevant messages.

Additional guidance:
- Small talk, acknowledgments, or “thank you” messages can still carry context — include them if they help preserve flow.
- If the user asks meta-questions (“что мы обсуждали?”, “а ты помнишь?”), include *all relevant prior topics* that could match the reference.
- If the conversation is long and multi-topic, select the last coherent segment (e.g. the last topic that lasted several turns).

If the new message is clearly unrelated to all previous topics, you may return an empty list.

💡 Tip: You can select **ranges** to keep the output compact when messages are consecutive.
Examples:
- Single messages: "5", "12"
- Ranges: "3-7" (includes 3,4,5,6,7)
- Mixed: ["0", "3-7", "12", "15-20"]

Return **only** JSON in this exact format:
{
  "relevant_indices": ["0", "3-7", "12"]
}`
}

func getDefaultModelSelectionPrompt() string {
	return `You are a model selection agent. Your job is to analyze a user task and recommend the most efficient AI model and reasoning effort — balancing quality, speed, and cost.

Available models for this tier (from MOST to LEAST capable), with AAI score (0–100), price per 1M tokens, and context window:

%s

Default reasoning effort for this tier: "%s"
Tier description: "%s"

%s

Core rules:
- Start from the **top model**, then **step down** to cheaper/faster ones if the task is simple, short, or routine.
- Stay within the user's tier whenever possible — they paid for its quality.
- Choose the **smallest capable model** that can reliably complete the task.
- Avoid using top-tier + high reasoning for trivial or conversational turns.
- If unsure, pick **medium reasoning**, not high.

When to downgrade:
- Task is short or factual (≤ 6 sentences)  
- Code edit is small/local  
- Simple math, rephrase, or obvious continuation  
- Light conversation or banter with Xi Manager  
- No high-stakes accuracy (e.g. legal/finance/medical)

When to stay high-tier:
- Multi-step reasoning, novel code, deep research  
- User requests "detailed", "in-depth", "thorough"  
- High-risk or high-importance tasks

Special cases:
- "Quick"/"fast" → prioritize speed + low reasoning  
- "Detailed"/"thorough" → prioritize quality + higher reasoning  
- Trolling/testing/nonsense → use trolling models (%s)

Temperature selection:
- Creative tasks (stories, brainstorming, casual chat): 0.8-1.2
- Balanced tasks (explanations, discussions): 0.7-1.0
- Precise tasks (code, math, facts, translation): 0.3-0.7
- If uncertain, use 1.0 as default

Heuristic:
Ask yourself: "Would an average competent model solve this correctly in one pass?"  
→ If yes, downgrade + low/medium reasoning.

Recent conversation context:
"""
%s
"""

New user task:
"""
%s
"""

Return only JSON in this format:
{
  "recommended_model": "exact model name from available list",
  "reasoning_effort": "low/medium/high",
  "task_complexity": "low/medium/high",
  "requires_speed": true/false,
  "requires_quality": true/false,
  "is_trolling": true/false,
  "temperature": 1.0
}`
}

func getDefaultSummarizationPrompt() string {
	return `You are a summarization agent. Your task is to condense the provided content while preserving all key information, context, and meaning.

The content may be a single message or a multi-turn conversation between a user and an assistant.

Content to summarize:
"""
%s
"""

Requirements:
1. Preserve all essential facts, decisions, and conclusions.
2. Keep important technical details, names, numbers, and specific references.
3. Maintain the logical flow and relationships between topics.
4. Remove redundancy, small talk, and irrelevant digressions.
5. Use clear and concise language.
6. The summary must be significantly shorter than the original (aim for about 30–50%% of the original length, unless the original text is already very short).
7. The summary must be in the same language as the original content.
8. Do NOT add any new information, examples, or assumptions that are not present in the original content.

Return ONLY the summary text, with no preamble, no quotes, and no formatting.`
}

func getDefaultPersonalizationValidationPrompt() string {
	return `You are a validation agent. Your task is to determine if the provided text is a self-description or personal information about the user.

Examples of valid self-descriptions:
- "I am a software engineer from Russia, I love coding and hiking"
- "My name is Ivan, I'm 25 years old student"
- "I work as a designer, passionate about art and music"
- "I'm a teacher who loves reading books and traveling"

Examples of invalid texts:
- "How to cook pasta?" (question, not self-description)
- "The weather is nice today" (general statement)
- "Buy groceries tomorrow" (task/reminder)
- "Python is a great language" (opinion about something else)

Analyze the following text and determine if it's a valid self-description.
Return your response in JSON format: {"confidence": 0.0-1.0}
Where confidence is how certain you are that this is a self-description (1.0 = definitely self-description, 0.0 = definitely not).

Text to analyze: %s

Return ONLY JSON, nothing else.`
}

func getDefaultWebSearchPrompt() string {
	return `You are a web search specialist agent. Your task is to find accurate and up-to-date information from the internet based on the user's query.

User's search query:
"""
%s
"""

Instructions:
1. Search for the most relevant and recent information related to the query.
2. Focus on authoritative and reliable sources.
3. Provide a comprehensive answer that directly addresses the user's question.
4. Include specific facts, dates, numbers, and citations where relevant.
5. If the query involves current events, prices, statistics, or time-sensitive data, prioritize the most recent information.
6. Structure your response clearly with key findings.
7. If you cannot find reliable information, clearly state that.

Return a clear, well-structured response with the search results.`
}

func getDefaultResponseLengthPrompt() string {
	return `You are a response length classification agent.
Your task is to analyze the user's message and choose how long the assistant's reply should be.

The user message may be written in ANY language.

User message:
"""
%s
"""

Response length categories:
1. very_brief   - 1–2 short sentences. Minimal answer: a fact, yes/no, or one simple rule.
2. brief        - 3–5 sentences. Short explanation without deep detail.
3. medium       - Normal answer: clear explanation with main details, but not extremely long.
4. detailed     - Extended explanation with examples, step-by-step reasoning, and important nuances.
5. very_detailed - Very long and deep answer: thorough analysis, multiple options, pros/cons, edge cases.

Detection guidelines (apply them in this order):

1. Explicit instructions about length:
   - If the user clearly asks for a short/brief/concise answer (in their language),
     prefer "very_brief" or "brief".
   - If the user clearly asks for a detailed/complete/in-depth answer (in their language),
     prefer "detailed" or "very_detailed".
   - If both short and detailed wishes appear, prefer the LONGER category.

2. Task type:
   - Simple factual questions like "who is...", "what is...", "when...", "where...",
     with no extra context, usually expect "very_brief" or "brief".
   - "Why" / "how" questions, requests for explanations, comparisons, analysis,
     normally require at least "medium", often "detailed".
   - Requests like "write code", "give a step-by-step tutorial", "design a plan"
     usually need at least "detailed", unless the user explicitly asks for brevity.

3. Message length and complexity:
   - Very short and simple messages (a few words, one clear question)
     usually expect "very_brief" or "brief", unless they clearly ask for deep detail.
   - Long messages with a lot of context, description, or multiple questions
     usually expect "medium" or "detailed".

4. Default behavior:
   - If there are no clear signals about the desired length and the message is not trivial,
     choose "medium".
   - Do NOT choose "very_detailed" unless the user explicitly or implicitly indicates
     they want a very deep and comprehensive answer.

Confidence calibration:
- 0.9–1.0 — there are explicit signals (about length or task type) and the choice is obvious.
- 0.7–0.89 — there are good indirect hints (question type, complexity, etc.).
- 0.5–0.69 — no strong signals; you choose the safest default (usually "medium").
- Do not use values below 0.5.

Important:
- Consider ONLY the text inside the triple quotes as the user message.
- Do not invent additional user requirements that are not present in the message.
- Do not output any explanation outside of the JSON.

Return ONLY JSON in this exact format:
{
  "length": "very_brief|brief|medium|detailed|very_detailed",
  "confidence": 0.0-1.0,
  "reasoning": "short explanation in English why this length was chosen"
}`
}