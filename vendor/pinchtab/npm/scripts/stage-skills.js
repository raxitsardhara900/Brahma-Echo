#!/usr/bin/env node

const fs = require('fs');
const path = require('path');

const SKILL_NAME = 'pinchtab';
const sourceDir = path.join(__dirname, '..', '..', 'skills', SKILL_NAME);
const targetDir = path.join(__dirname, '..', 'skills', SKILL_NAME);

function copyDirSync(src, dest) {
  fs.mkdirSync(dest, { recursive: true });

  const entries = fs.readdirSync(src, { withFileTypes: true });
  for (const entry of entries) {
    const srcPath = path.join(src, entry.name);
    const destPath = path.join(dest, entry.name);

    if (entry.isDirectory()) {
      copyDirSync(srcPath, destPath);
      continue;
    }

    fs.copyFileSync(srcPath, destPath);
  }
}

function stageSkills() {
  if (!fs.existsSync(sourceDir)) {
    throw new Error(`source skill directory not found: ${sourceDir}`);
  }

  fs.rmSync(targetDir, { recursive: true, force: true });
  copyDirSync(sourceDir, targetDir);
}

if (require.main === module) {
  stageSkills();
  console.log(`Staged ${SKILL_NAME} skill into npm package`);
}

module.exports = { stageSkills };
