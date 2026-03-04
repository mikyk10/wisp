import { chromium } from 'playwright';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

const svgFile = path.resolve(__dirname, 'src/logo-epd-error.svg');
const outputFile = path.resolve(__dirname, '../../..', 'api/app/domain/catalog/error-icon.png');

const browser = await chromium.launch();
const page = await browser.newPage();

// Set viewport to square for icon
await page.setViewportSize({ width: 120, height: 120 });
await page.goto(`file://${svgFile}`);
await page.waitForLoadState('networkidle');
await page.screenshot({ path: outputFile });

console.log(`✓ Generated ${outputFile}`);

await browser.close();
