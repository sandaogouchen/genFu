# frontend-web/

> **分析时间**: 2026-03-31T20:00:00+08:00
> **源分支**: `main` | **分析提交**: `04bd5e16fa7d`
> **直接源文件数**: 9 (排除 package-lock.json)
> **直接子目录**: public, src

## 目录职责概述

React/TypeScript 前端应用，使用 Vite 构建、Tailwind CSS 样式。提供投资分析平台的 Web UI，包括对话、选股、决策、行情、新闻等页面。

## 文件分析

### §1 `package.json`
- **类型**: 依赖配置
- **职责**: 声明前端依赖（React 18, Zustand, Tailwind, Vite 等）
### §2 `vite.config.ts`
- **类型**: 构建配置
- **职责**: Vite 构建配置，含 API 代理设置
### §3 `tailwind.config.js`
- **类型**: 样式配置
### §4 `tsconfig.json`
- **类型**: TypeScript 配置
### §5 `index.html`
- **类型**: 入口HTML
### §6 `.eslintrc.cjs`
- **类型**: Lint 配置
### §7 `postcss.config.js`
- **类型**: PostCSS 配置
### §8 `README.md`
- **类型**: 前端文档

## 对外暴露接口

- 构建产物（dist/）部署为静态网站
- 通过 API 代理连接后端
