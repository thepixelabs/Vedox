---
title: Unicode Content Validation
type: doc
status: draft
date: 2026-04-07
tags:
  - unicode
  - i18n
---

## Unicode Content

Japanese: こんにちは世界 — The quick brown fox jumps over the lazy dog.

Chinese: 你好，世界！Vedox 是一个本地优先的文档管理系统。

Arabic (RTL): مرحبا بالعالم — هذا نص بالعربية.

Korean: 안녕하세요 세계! 빠른 갈색 여우가 게으른 개 위를 뛰어넘었습니다.

Russian: Привет, мир! Быстрая бурая лиса перепрыгнула через ленивую собаку.

Greek: Γεια σου κόσμε! Η γρήγορη καφέ αλεπού πηδά πάνω από τον τεμπέλη σκύλο.

Emoji in prose: The deployment pipeline 🚀 succeeded. All tests passed ✅. No regressions found 🎉.

Special characters: `<script>alert('xss')</script>` should be preserved as literal code.

Math symbols: E = mc², f(x) = x² + 2x + 1, π ≈ 3.14159265358979, ∑(n=1 to ∞) 1/n² = π²/6

Currency: £ € $ ¥ ₹ ₩ ₿

Typography: "curly quotes" and 'single quotes' and em-dash — and en-dash – and ellipsis…

Zero-width and combining characters: café (e + combining accent), naïve, résumé.
