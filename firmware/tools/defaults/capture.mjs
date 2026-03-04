import { chromium } from 'playwright';
import path from 'path';

const [,, src, width, height, out] = process.argv;

const browser = await chromium.launch();
const page = await browser.newPage();
await page.setViewportSize({ width: parseInt(width), height: parseInt(height) });
await page.goto(`file://${path.resolve(src)}`);
await page.waitForLoadState('networkidle');
await page.screenshot({ path: out });
await browser.close();
