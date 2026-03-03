---
version: v2
stage: tagging
#provider: ollama_local
#model: qwen2.5:latest
provider: openai
model: gpt-5-nano
---
Output up to {{.MaxTags}} tags for the photo description below.
Each tag must be a single complete lowercase English word (a-z only, no compound words).
Separate tags with spaces. Output only the tags, nothing else.

Description: {{.Description}}

Tags:
