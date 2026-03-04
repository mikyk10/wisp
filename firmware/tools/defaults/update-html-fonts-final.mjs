import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

// Fonts from api/app/domain/improc/fonts/
const FONTS = [
  { name: 'Iansui', file: '../../../api/app/domain/improc/fonts/Iansui-Regular.ttf' },
  { name: 'Unkempt', file: '../../../api/app/domain/improc/fonts/Unkempt-Regular.ttf' }
];

// Generate @font-face CSS rules
function generateFontFace() {
  let css = '';
  for (const font of FONTS) {
    const fontPath = path.resolve(__dirname, font.file);
    const fontData = fs.readFileSync(fontPath);
    const base64 = fontData.toString('base64');
    css += `    @font-face {
      font-family: '${font.name}';
      src: url('data:font/ttf;base64,${base64}');
    }\n`;
  }
  return css;
}

// HTML files to update
const HTML_FILES = ['src/epd7in3e.html', 'src/epd4in0e.html', 'src/epd13in3e.html'];

const fontFaceCSS = generateFontFace();

for (const htmlFile of HTML_FILES) {
  const filePath = path.resolve(__dirname, htmlFile);
  let html = fs.readFileSync(filePath, 'utf-8');

  // Find the <style> tag and inject fonts
  const styleMatch = html.match(/<style>([\s\S]*?)<\/style>/);

  if (styleMatch) {
    const styleContent = styleMatch[1];
    // Remove existing @font-face rules if any
    const cleanedStyle = styleContent.replace(/@font-face\s*\{[^}]*\}/g, '');

    // Improve .reason style: slightly larger size + lighter/brighter red
    let improvedStyle = cleanedStyle
      .replace(
        /\.reason\s*\{\s*font-size:\s*12px;/,
        '.reason {\n      font-size: 13px;'
      )
      .replace(
        /color:\s*#ff4444;/,
        'color: #ff8888;'
      );

    const newStyle = `<style>\n${fontFaceCSS}\n${improvedStyle}<\/style>`;
    html = html.replace(/<style>[\s\S]*?<\/style>/, newStyle);

    // Set Iansui as default for body
    html = html.replace(
      /font-family:\s*-apple-system,\s*BlinkMacSystemFont,\s*'Segoe UI',\s*'Helvetica Neue',\s*sans-serif/g,
      "font-family: 'Iansui', -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Helvetica Neue', sans-serif"
    );

    // Add Unkempt specifically for .title (WiSP only)
    html = html.replace(
      /\.title\s*\{/,
      ".title {\n      font-family: 'Unkempt';"
    );

    fs.writeFileSync(filePath, html);
    console.log(`✓ Updated ${htmlFile}`);
  } else {
    console.error(`✗ Could not find <style> tag in ${htmlFile}`);
  }
}

console.log('\nDone! Now run: make');
