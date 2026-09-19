---
title: Zero-Downtime Deploys
theme: terminal
author: Platform Team
date: 2026-09-19
aspectRatio: "16:9"
transition: fade
---

# Zero-Downtime Deploys

Rolling out a release without dropping a request

---

## The Setup

- Four app servers behind one load balancer
- A release goes out one server at a time
- The other three keep serving while it restarts

---

<!--
layout: ./slides/RollingDeploy.jsx
-->

# The Rollout

::caption
Each server drains, restarts on the new version, and rejoins before the next one starts.

---

## What It Bought Us

```component ./charts/LatencyDrop.jsx
{ "before": 412, "after": 88, "unit": "ms", "label": "p95 latency" }
```

<!-- pause -->

- No error budget spent on the deploy itself
- No page from an on-call engineer

---

<!--
layout: two-column
-->

## Two Regions, Two Numbers

::left

```component ./charts/LatencyDrop.jsx
{ "before": 530, "after": 120, "unit": "ms", "label": "us-east p95" }
```

::right

```component ./charts/LatencyDrop.jsx
{ "before": 410, "after": 95, "unit": "ms", "label": "eu-west p95" }
```

---

# Ship Without the 3am Page

The whole deck, including these components, is in `examples/components/`
