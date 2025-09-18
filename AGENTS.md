# Agent Guidelines

## YAGNI Principle (You Aren't Gonna Need It)

When working with AI agents on this project, **always** follow the YAGNI principle:

- **Only implement what is actually required** - Don't add features or fields "just in case"
- **Start with minimal compliance** - Meet the basic contract requirements first
- **Add complexity incrementally** - Only when there's a proven need
- **Avoid over-engineering** - Don't implement optional features unless explicitly requested

### For Cluster API Provider Development

- **Mandatory fields only initially**: Focus on the required contract fields first
- **Optional features later**: Features like failure domains, templates, and clusterctl support should only be added when needed
- **Simple implementations**: Start with the simplest working implementation

This helps keep the codebase lean, maintainable, and focused on actual requirements rather than speculative features.
