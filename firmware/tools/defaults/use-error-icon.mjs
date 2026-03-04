import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

const HTML_FILES = ['src/epd7in3e.html', 'src/epd4in0e.html', 'src/epd13in3e.html'];

for (const htmlFile of HTML_FILES) {
  const filePath = path.resolve(__dirname, htmlFile);
  let html = fs.readFileSync(filePath, 'utf-8');

  // Replace logo-epd.svg with logo-epd-error.svg
  html = html.replace(/logo-epd\.svg/g, 'logo-epd-error.svg');

  fs.writeFileSync(filePath, html);
  console.log(`✓ Updated ${htmlFile} to use logo-epd-error.svg`);
}

console.log('\nDone! Now run: make');
