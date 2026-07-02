---
name: rft-fuzzy-mv
description: Make refactoring implementation more exaustive, complete and stable by testing operations on repositories in the wild and checking wether rft reacts the expected way.
---

# Restrictions
Only run this skill on ephemeral environments. If it looks like a personal computer or a VPS, refuse it.

# The process

- Choose one project from projects.md
- Clone it in a temporary location
- Run it's tests to check wether it is in a non-broken state
- Test the mv operation on it in a random configuration
- See how rft reacts to the operation, wether it makes sense and wether the project broke because of the operation.
- If it broke
  - Debug why it broke
  - Create a mv test fixture that hits the edge case
  - Fix the implementation to deal with the issue
  - Suggest a more clever but simple abstraction structure to support the middle ground if necessary
- Reset the project you cloned to the original state for the next run
