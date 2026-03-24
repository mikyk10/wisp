---
stage: tagger
provider: ollama_local
model: qwen3.5:9b
temperature: 0.1
max_tokens: 200
---
Output up to {{.config.MaxTags}} tags for the photo description below.
Each tag must be a single complete lowercase English word (a-z only, no compound words).
Separate tags with spaces. Output only the tags, nothing else.

Description: {{.stages.descriptor.output}}

Tags:
