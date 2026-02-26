// use this script to generate instructions for AI Agents on how to use dalforge tool.

const fs = require('fs');
const path = require('path');

/* ========================================================================
   CONFIGURATION
   ======================================================================== */

// 1. Files/Folders to concatenate into 'bigapi-context.md'
const CONTENT_TARGETS = [
  'instructions-template.md',
  'example/user.yaml',
  'example/post.yaml',
  'example/dal/serverprovider.gen.go',
  'example/dal/telemetry.gen.go',
  'example/dal/telemetryprovider.gen.go',
  'example/dal/rediscacheprovider.gen.go',  
  'example/dal/user.gen.go',  
];

// 2. Output filenames
const CONTENT_OUTPUT_FILE = 'dalforge-instructions.md';

// 3. Folders to ignore in the Tree View
const IGNORE_DIRS = new Set([
  'node_modules', 
  '.git',   
  'dist', 
  'build', 
  'coverage',
  '.vscode'
]);

// 4. File extensions to include in the Content Dump
const VALID_EXTENSIONS = ['.ts', '.tsx', '.js', '.json', '.md', '.go', '.yaml', '.yml'];

/* ========================================================================
   PART 2: GENERATE FILE CONTENT DUMP
   ======================================================================== */

function getAllFiles(dirPath, arrayOfFiles) {
  let files;
  try {
    files = fs.readdirSync(dirPath);
  } catch(e) { return []; }

  arrayOfFiles = arrayOfFiles || [];

  files.forEach(function(file) {
    const fullPath = path.join(dirPath, file);
    if (fs.statSync(fullPath).isDirectory()) {
        // Recursive dive, respecting ignores
        if (!IGNORE_DIRS.has(file)) {
            arrayOfFiles = getAllFiles(fullPath, arrayOfFiles);
        }
    } else {
      if (VALID_EXTENSIONS.includes(path.extname(fullPath))) {
        arrayOfFiles.push(fullPath);
      }
    }
  });

  return arrayOfFiles;
}

let outputContent = '';
let processedFiles = 0;

console.log(`\n📄 Gathering File Contents...`);

CONTENT_TARGETS.forEach((targetPath) => {
  const fullPath = path.join(__dirname, targetPath);
  
  if (fs.existsSync(fullPath)) {
    const stat = fs.statSync(fullPath);
    
    if (stat.isDirectory()) {
      const files = getAllFiles(fullPath, []);
      files.forEach(f => {
        const relativePath = path.relative(__dirname, f);
        const content = fs.readFileSync(f, 'utf8');
        outputContent += `\n### ${relativePath}\n\`\`\`typescript\n${content}\n\`\`\`\n---\n`;
        processedFiles++;
      });
      console.log(`   ✓ Directory: ${targetPath} (${files.length} files)`);
    } else {
      const content = fs.readFileSync(fullPath, 'utf8');
      outputContent += `\n### ${targetPath}\n\`\`\`typescript\n${content}\n\`\`\`\n---\n`;
      console.log(`   ✓ File: ${targetPath}`);
      processedFiles++;
    }
  } else {
    console.warn(`   ! Skipped (not found): ${targetPath}`);
  }
});

fs.writeFileSync(CONTENT_OUTPUT_FILE, outputContent);
console.log(`   ✓ Saved ${processedFiles} files to: ${CONTENT_OUTPUT_FILE}`);
console.log(`\n✨ All done! Upload '${CONTENT_OUTPUT_FILE}' to the chat.`);