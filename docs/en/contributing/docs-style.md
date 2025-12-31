# Documentation Standards

---

## Overview

This document defines documentation writing standards for the WES project.

---

## Document Organization

### Directory Structure

```
docs/en/
├── getting-started/   # Getting Started
├── concepts/          # Core Concepts
├── tutorials/         # Tutorials
├── how-to/            # How-to Guides
├── reference/         # Reference Documentation
├── contributing/      # Contributing Guide
└── support/           # Support Information
```

### Document Types

| Type | Purpose | Characteristics |
|------|---------|-----------------|
| Concept Documents | Explain what and why | Theoretical, background knowledge |
| Tutorials | Step-by-step teaching | Clear steps, followable |
| How-to Guides | Complete specific tasks | Goal-oriented, practical |
| Reference Documentation | Detailed technical specifications | Comprehensive, accurate |

---

## Writing Standards

### Headings

- Use `#` for first-level headings
- Heading levels should not exceed 4
- Headings should be concise and clear

```markdown
# First-Level Heading

## Second-Level Heading

### Third-Level Heading

#### Fourth-Level Heading
```

### Paragraphs

- Empty line between paragraphs
- Each paragraph expresses one main point
- Keep paragraphs short

### Lists

Unordered list:
```markdown
- Item 1
- Item 2
- Item 3
```

Ordered list:
```markdown
1. Step 1
2. Step 2
3. Step 3
```

### Code

Inline code:
```markdown
Use `wes-node start` to start the node.
```

Code block:
````markdown
```go
func main() {
    fmt.Println("Hello, WES!")
}
```
````

### Tables

```markdown
| Column 1 | Column 2 | Column 3 |
|----------|----------|----------|
| Value 1  | Value 2  | Value 3  |
```

### Links

Internal links:
```markdown
[Installation Guide](./installation.md)
[Core Concepts](../concepts/)
```

External links:
```markdown
[GitHub](https://github.com/weisyn/weisyn)
```

---

## Document Templates

### Concept Document Template

```markdown
# [Concept Name]

---

## Overview

[Brief introduction to this concept]

---

## Why [Concept Name]?

[Explain background and motivation]

---

## Core Capabilities

### Capability 1

[Detailed description]

### Capability 2

[Detailed description]

---

## Configuration

[Configuration parameter table]

---

## Related Documentation

- [Related Document 1](./related1.md)
- [Related Document 2](./related2.md)
```

### How-to Guide Template

```markdown
# [Operation Name]

---

## Overview

[Brief introduction to this operation]

---

## Prerequisites

- [Prerequisite 1]
- [Prerequisite 2]

---

## Steps

### Step 1: [Step Name]

[Detailed description]

```bash
# Command example
```

### Step 2: [Step Name]

[Detailed description]

---

## FAQ

### Q: [Question]

A: [Answer]

---

## Related Documentation

- [Related Document](./related.md)
```

---

## Style Guide

### Language

- Use English
- Keep technical terms consistent
- Avoid colloquial expressions

### Tone

- Use second person ("you")
- Keep professional but friendly
- Avoid being too formal

### Terminology

- Explain terms when first appearing
- Use standard terms from glossary
- Keep English terms as-is or add Chinese notes

---

## Review Checklist

- [ ] Title is clear and accurate
- [ ] Content is complete without omissions
- [ ] Code examples are runnable
- [ ] Links are valid
- [ ] Format is correct
- [ ] Terminology is consistent
- [ ] No spelling errors

---

## Related Documentation

- [Development Environment Setup](./development-setup.md) - Environment configuration
- [Code Standards](./code-style.md) - Code standards
- [Glossary](../concepts/glossary.md) - Term definitions

