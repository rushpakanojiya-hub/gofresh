/**
 * Bulk-imports categories + products (with images) into the ecommerce backend,
 * using the product data extracted from the Blinkit Flutter app.
 *
 * USAGE (from this scripts/ folder):
 *   node bulk_import_products.js --token YOUR_ADMIN_TOKEN --images "C:\Users\Administrator\Desktop\Blinkit\assets"
 *
 * Where to get YOUR_ADMIN_TOKEN:
 *   1. Open the admin panel in Chrome (localhost:5173) and log in.
 *   2. Press F12 -> Application tab -> Local Storage -> http://localhost:5173
 *   3. Copy the value of the "admin_token" key.
 *
 * Where --images points:
 *   The folder that contains "images/Fruits", "images/Chocolate", "images/cloths items", etc.
 *   i.e. the Blinkit project's "assets" folder.
 *
 * Requires Node.js 18+ (uses global fetch/FormData/Blob).
 */

const fs = require('fs');
const path = require('path');

const BASE_URL = 'https://gofresh-evl7.onrender.com/api/v1';

function parseArgs() {
  const args = process.argv.slice(2);
  const out = { dataFile: 'blinkit_products.json' };
  for (let i = 0; i < args.length; i++) {
    if (args[i] === '--token') out.token = args[++i];
    if (args[i] === '--images') out.imagesDir = args[++i];
    if (args[i] === '--data') out.dataFile = args[++i];
  }
  return out;
}

async function main() {
  const { token, imagesDir, dataFile } = parseArgs();
  if (!token || !imagesDir) {
    console.error('Usage: node bulk_import_products.js --token YOUR_ADMIN_TOKEN --images "path\\to\\Blinkit\\assets" [--data other_products.json]');
    process.exit(1);
  }
  if (!fs.existsSync(imagesDir)) {
    console.error(`Images folder not found: ${imagesDir}`);
    process.exit(1);
  }

  const products = JSON.parse(fs.readFileSync(path.join(__dirname, dataFile), 'utf-8'));
  console.log(`Loaded ${products.length} products from ${dataFile}`);

  const headers = { Authorization: `Bearer ${token}` };

  // ---- Step 1: Create categories ----
  const categoryNames = [...new Set(products.map((p) => p.category))];
  const categoryIdByName = {};

  console.log(`\nCreating ${categoryNames.length} categories...`);
  for (const name of categoryNames) {
    try {
      const res = await fetch(`${BASE_URL}/admin/categories`, {
        method: 'POST',
        headers: { ...headers, 'Content-Type': 'application/json' },
        body: JSON.stringify({ name }),
      });
      const data = await res.json();
      if (res.ok) {
        categoryIdByName[name] = data.id;
        console.log(`  OK  ${name} -> id ${data.id}`);
      } else if (res.status === 409) {
        // Already exists - fetch categories list to find its id
        const listRes = await fetch(`${BASE_URL}/categories`);
        const listData = await listRes.json();
        const existing = (listData.categories || []).find((c) => c.name === name);
        if (existing) {
          categoryIdByName[name] = existing.id;
          console.log(`  SKIP ${name} (already exists) -> id ${existing.id}`);
        } else {
          console.error(`  FAIL ${name}: conflict but could not find existing id`);
        }
      } else {
        console.error(`  FAIL ${name}:`, data.error || res.status);
      }
    } catch (e) {
      console.error(`  FAIL ${name}:`, e.message);
    }
  }

  // ---- Step 2: Upload image + create each product ----
  console.log(`\nCreating ${products.length} products...`);
  let ok = 0, fail = 0, skippedImage = 0;

  for (const p of products) {
    const categoryId = categoryIdByName[p.category];
    if (!categoryId) {
      console.error(`  FAIL ${p.name}: no category id for "${p.category}"`);
      fail++;
      continue;
    }

    // image field in JSON looks like: assets/images/Fruits/Apple.png
    const relativePath = p.image.startsWith('assets/') ? p.image.slice('assets/'.length) : p.image;
    const localImagePath = path.join(imagesDir, relativePath);

    let imageUrl = '';
    if (fs.existsSync(localImagePath)) {
      try {
        const fileBuffer = fs.readFileSync(localImagePath);
        const ext = path.extname(localImagePath).toLowerCase();
        const mime = ext === '.jpg' || ext === '.jpeg' ? 'image/jpeg' : ext === '.webp' ? 'image/webp' : 'image/png';
        const blob = new Blob([fileBuffer], { type: mime });
        const form = new FormData();
        form.append('image', blob, path.basename(localImagePath));

        const uploadRes = await fetch(`${BASE_URL}/upload`, {
          method: 'POST',
          headers, // don't set Content-Type, fetch sets multipart boundary automatically
          body: form,
        });
        const uploadData = await uploadRes.json();
        if (uploadRes.ok) {
          imageUrl = uploadData.url || uploadData.image_url || '';
        } else {
          console.warn(`  WARN ${p.name}: image upload failed (${uploadData.error || uploadRes.status}), creating without image`);
        }
      } catch (e) {
        console.warn(`  WARN ${p.name}: image upload error (${e.message}), creating without image`);
      }
    } else {
      skippedImage++;
    }

    try {
      const res = await fetch(`${BASE_URL}/admin/products`, {
        method: 'POST',
        headers: { ...headers, 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: p.name,
          description: `${p.name} - ${p.unit}`,
          price: p.price,
          image_url: imageUrl,
          category_id: categoryId,
          stock: 50,
        }),
      });
      const data = await res.json();
      if (res.ok) {
        ok++;
        console.log(`  OK  ${p.name}`);
      } else {
        fail++;
        console.error(`  FAIL ${p.name}:`, data.error || res.status);
      }
    } catch (e) {
      fail++;
      console.error(`  FAIL ${p.name}:`, e.message);
    }
  }

  console.log(`\nDone. Created: ${ok}, Failed: ${fail}, Images not found locally: ${skippedImage}`);
}

main();
