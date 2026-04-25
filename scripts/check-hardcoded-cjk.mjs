#!/usr/bin/env node

import fs from "node:fs"
import path from "node:path"

const targetDir = process.argv[2] || "web-admin/src"
const whitelistPath = process.argv[3] || "web-admin/cjk-whitelist.txt"
const cjkPattern = /[\u3400-\u9fff\uf900-\ufaff]/

const skipDirNames = new Set([
  ".git",
  "dist",
  "node_modules",
  "coverage",
  "locales",
])

const whitelistPatterns = loadWhitelistPatterns(whitelistPath)
const findings = []

walk(targetDir)

if (findings.length > 0) {
  console.error("Hardcoded CJK text detected outside whitelist:")
  findings.forEach((item) => {
    console.error(`${item.file}:${item.line}: ${item.text}`)
  })
  process.exit(1)
}

console.log(`CJK scan passed: ${targetDir}`)

function loadWhitelistPatterns(file) {
  if (!fs.existsSync(file)) {
    return []
  }
  return fs
    .readFileSync(file, "utf8")
    .split(/\r?\n/g)
    .map((line) => line.trim())
    .filter((line) => line.length > 0 && !line.startsWith("#"))
    .map((line) => {
      try {
        return new RegExp(line)
      } catch (error) {
        throw new Error(`Invalid whitelist regex: ${line}\n${error instanceof Error ? error.message : String(error)}`)
      }
    })
}

function walk(currentPath) {
  if (!fs.existsSync(currentPath)) {
    return
  }
  const stat = fs.statSync(currentPath)
  if (stat.isDirectory()) {
    const base = path.basename(currentPath)
    if (skipDirNames.has(base)) {
      return
    }
    for (const name of fs.readdirSync(currentPath)) {
      walk(path.join(currentPath, name))
    }
    return
  }

  if (!stat.isFile()) {
    return
  }

  const normalizedPath = currentPath.replaceAll(path.sep, "/")
  if (isWhitelisted(normalizedPath)) {
    return
  }

  const ext = path.extname(currentPath).toLowerCase()
  if (![".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".css", ".mdx"].includes(ext)) {
    return
  }

  const lines = fs.readFileSync(currentPath, "utf8").split(/\r?\n/g)
  lines.forEach((line, index) => {
    if (cjkPattern.test(line)) {
      findings.push({
        file: normalizedPath,
        line: index + 1,
        text: line.trim(),
      })
    }
  })
}

function isWhitelisted(normalizedPath) {
  return whitelistPatterns.some((pattern) => pattern.test(normalizedPath))
}

