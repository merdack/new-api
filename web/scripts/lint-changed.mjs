/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { spawnSync } from 'node:child_process'

const base = process.argv[2] || process.env.LINT_BASE_REF || 'origin/main'
const diff = spawnSync(
  'git',
  ['diff', '--name-only', '--diff-filter=ACMR', `${base}...HEAD`, '--', 'web/src'],
  { cwd: '..', encoding: 'utf8' }
)

if (diff.status !== 0) {
  process.stderr.write(diff.stderr || 'Unable to determine changed files.\n')
  process.exit(diff.status || 1)
}

const files = diff.stdout
  .split('\n')
  .filter((file) => /\.(?:js|jsx|ts|tsx)$/.test(file))
  .map((file) => file.replace(/^web\//, ''))

if (files.length === 0) {
  console.log('No changed frontend source files to lint.')
  process.exit(0)
}

const lint = spawnSync(
  process.platform === 'win32' ? 'bunx.cmd' : 'bunx',
  ['oxlint', '-c', '.oxlintrc.json', ...files],
  { stdio: 'inherit' }
)
if (lint.error) {
  process.stderr.write(`${lint.error.message}\n`)
}
process.exit(lint.status ?? 1)
