import { chromium } from 'playwright';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

const svgFile = path.resolve(__dirname, 'src/logo-epd-error.svg');
const outputFile = path.resolve(__dirname, 'src/logo-epd-error.png');

const browser = await chromium.launch();
const page = await browser.newPage();

// Set viewport to match SVG size
await page.setViewportSize({ width: 120, height: 120 });
await page.goto(`file://${svgFile}`);
await page.waitForLoadState('networkidle');
await page.screenshot({ path: outputFile });

console.log(`✓ Generated ${outputFile}`);

// Update HTML files to use PNG instead of SVG
const fs = await import('fs');
const HTML_FILES = ['src/epd7in3e.html', 'src/epd4in0e.html', 'src/epd13in3e.html'];

for (const htmlFile of HTML_FILES) {
  const filePath = path.resolve(__dirname, htmlFile);
  let html = fs.readFileSync(filePath, 'utf-8');
  html = html.replace(/logo-epd-error\.svg/g, 'logo-epd-error.png');
  fs.writeFileSync(filePath, html);
  console.log(`✓ Updated ${htmlFile} to use logo-epd-error.png`);
}

await browser.close();

console.log('\nDone! Now run: make');
