import fs from 'fs';
import path from 'path';

// Fonts from api/app/domain/improc/fonts/
const FONTS = [
  { name: 'Iansui-Regular', file: '../../../api/app/domain/improc/fonts/Iansui-Regular.ttf' },
  { name: 'Unkempt-Regular', file: '../../../api/app/domain/improc/fonts/Unkempt-Regular.ttf' }
];

// Generate @font-face CSS rules
function generateFontFace() {
  let css = '';
  for (const font of FONTS) {
    const fontPath = path.resolve(font.file);
    const fontData = fs.readFileSync(fontPath);
    const base64 = fontData.toString('base64');
    css += `  @font-face {
    font-family: '${font.name}';
    src: url('data:font/ttf;base64,${base64}');
  }\n`;
  }
  return css;
}

console.log('Generated @font-face rules:\n');
console.log(generateFontFace());
