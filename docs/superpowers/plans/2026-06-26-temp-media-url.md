# Temp Media URL Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an authenticated temp image upload endpoint that returns a public backend URL, then make `veo_json` upload local reference images first and send only URLs in `Ingredients_images`.

**Architecture:** The backend stores uploaded temp images on a local bind-mounted directory and exposes them through a public read-only route under `/backend-api/media/tmp/:name`. The frontend `veo_json` path uploads any non-public reference image before video creation and uses the returned URL instead of base64.

**Tech Stack:** Go, Gin, GORM, Next.js, TypeScript, Axios, Docker Compose

## Global Constraints

- Follow existing Gin handler/service/repository boundaries; avoid unrelated refactors.
- Keep response shape consistent with `{ code, data, msg }`.
- Only `veo_json` uses the new temp upload flow.
- Temp file URLs must be publicly reachable via the existing backend domain path.
- Do not change unrelated image/video generation branches.

---
