import sys, math
from PIL import Image, ImageDraw, ImageFilter, ImageChops

SS = 4  # supersampling
S = 1024 * SS
out = sys.argv[1]

def lerp(a, b, t): return tuple(int(a[i] + (b[i] - a[i]) * t) for i in range(len(a)))

def gradient(size, stops, angle_deg=135):
    w, h = size
    img = Image.new('RGBA', size)
    px = img.load()
    a = math.radians(angle_deg)
    dx, dy = math.sin(a), -math.cos(a)  # CSS angle: 135 = toward bottom right
    # project corners to normalise
    proj = [x * dx + y * dy for x in (0, w) for y in (0, h)]
    lo, hi = min(proj), max(proj)
    row = Image.new('RGBA', (w, h))
    for y in range(0, h, 4):
        for x in range(0, w, 4):
            t = ((x * dx + y * dy) - lo) / (hi - lo)
            for i in range(len(stops) - 1):
                p0, c0 = stops[i]; p1, c1 = stops[i + 1]
                if t <= p1:
                    c = lerp(c0, c1, (t - p0) / (p1 - p0)); break
            else:
                c = stops[-1][1]
            for yy in range(y, min(y + 4, h)):
                for xx in range(x, min(x + 4, w)):
                    px[xx, yy] = c
    return img

def background(size):
    bg = gradient(size, [(0, (79, 70, 229, 255)), (0.55, (124, 58, 237, 255)), (1, (192, 38, 211, 255))])
    # soft light from the top left
    glow = Image.new('L', size, 0)
    d = ImageDraw.Draw(glow)
    w, h = size
    d.ellipse([-w * 0.35, -h * 0.45, w * 0.75, h * 0.55], fill=110)
    glow = glow.filter(ImageFilter.GaussianBlur(w * 0.12))
    white = Image.new('RGBA', size, (255, 255, 255, 255))
    bg = Image.composite(white, bg, glow.point(lambda v: v * 0.35))
    return bg

def glyph(size, mono=False):
    """The page and the lens, on transparent, scaled to size."""
    w, h = size
    u = w / 1024
    layer = Image.new('RGBA', size, (0, 0, 0, 0))
    # shadow under the page
    shadow = Image.new('L', size, 0)
    sd = ImageDraw.Draw(shadow)
    px0, py0, px1, py1 = 250 * u, 190 * u, 690 * u, 790 * u
    fold = 150 * u
    sd.rounded_rectangle([px0 + 10 * u, py0 + 36 * u, px1 + 10 * u, py1 + 36 * u], radius=60 * u, fill=120)
    # the shadow loses the folded corner too
    sd.polygon([(px1 - fold + 10 * u, py0 + 36 * u - 2), (px1 + 12 * u, py0 + 36 * u - 2), (px1 + 12 * u, py0 + fold + 36 * u)], fill=0)
    shadow = shadow.filter(ImageFilter.GaussianBlur(40 * u))
    if not mono:
        layer.paste((30, 20, 90, 255), (0, 0), shadow.point(lambda v: v * 0.55))
    d = ImageDraw.Draw(layer)
    page = (255, 255, 255, 255)
    # page body with the top-right corner cut for the fold
    d.rounded_rectangle([px0, py0, px1, py1], radius=56 * u, fill=page)
    d.polygon([(px1 - fold, py0 - 2), (px1 + 2, py0 - 2), (px1 + 2, py0 + fold)], fill=(0, 0, 0, 0))
    # the fold
    fold_col = (199, 210, 254, 255) if not mono else (255, 255, 255, 255)
    # the turned-down flap: the lower left half of the corner square
    d.polygon([(px1 - fold, py0), (px1 - fold, py0 + fold - 24 * u), (px1 - fold + 24 * u, py0 + fold), (px1, py0 + fold)], fill=fold_col)
    # text lines
    line = (165, 180, 252, 255) if not mono else (0, 0, 0, 0)
    head = (99, 102, 241, 255) if not mono else (0, 0, 0, 0)
    x0 = px0 + 70 * u
    d.rounded_rectangle([x0, 300 * u, x0 + 190 * u, 336 * u], radius=18 * u, fill=head)
    for i, ln in enumerate([300, 260, 300, 210]):
        y = 400 * u + i * 70 * u
        d.rounded_rectangle([x0, y, x0 + ln * u, y + 28 * u], radius=14 * u, fill=line)
    # the lens: white ring, deep glass, highlight
    cx, cy, r = 650 * u, 650 * u, 205 * u
    ring_shadow = Image.new('L', size, 0)
    ImageDraw.Draw(ring_shadow).ellipse([cx - r, cy - r + 24 * u, cx + r, cy + r + 24 * u], fill=150)
    ring_shadow = ring_shadow.filter(ImageFilter.GaussianBlur(34 * u))
    if not mono:
        layer.paste((30, 15, 80, 255), (0, 0), ImageChops.multiply(ring_shadow, Image.new('L', size, 140)))
    d = ImageDraw.Draw(layer)
    d.ellipse([cx - r, cy - r, cx + r, cy + r], fill=(255, 255, 255, 255))
    if mono:
        g = 120 * u
        d.ellipse([cx - g, cy - g, cx + g, cy + g], fill=(0, 0, 0, 0))
        return layer
    gr = r - 46 * u
    glass = gradient((int(2 * gr), int(2 * gr)), [(0, (67, 56, 202, 255)), (1, (30, 27, 75, 255))], 160)
    m = Image.new('L', glass.size, 0)
    ImageDraw.Draw(m).ellipse([0, 0, glass.size[0] - 1, glass.size[1] - 1], fill=255)
    layer.paste(glass, (int(cx - gr), int(cy - gr)), m)
    d = ImageDraw.Draw(layer)
    # inner iris ring and pupil
    ir = gr * 0.58
    d.ellipse([cx - ir, cy - ir, cx + ir, cy + ir], outline=(129, 140, 248, 255), width=int(14 * u))
    pr = gr * 0.28
    d.ellipse([cx - pr, cy - pr, cx + pr, cy + pr], fill=(15, 12, 45, 255))
    # highlight
    hl = Image.new('L', size, 0)
    ImageDraw.Draw(hl).ellipse([cx - gr * 0.58, cy - gr * 0.66, cx - gr * 0.22, cy - gr * 0.38], fill=210)
    hl = hl.filter(ImageFilter.GaussianBlur(3 * u))
    layer.paste((255, 255, 255, 255), (0, 0), hl)
    d.ellipse([cx + gr * 0.30, cy + gr * 0.28, cx + gr * 0.48, cy + gr * 0.46], fill=(255, 255, 255, 140))
    return layer

def rounded(img, radius):
    m = Image.new('L', img.size, 0)
    ImageDraw.Draw(m).rounded_rectangle([0, 0, img.size[0] - 1, img.size[1] - 1], radius=radius, fill=255)
    out = Image.new('RGBA', img.size, (0, 0, 0, 0))
    out.paste(img, (0, 0), m)
    return out

def down(img, n): return img.resize((n, n), Image.LANCZOS)

# App icon: full bleed square (stores round it themselves) and a rounded desktop one.
bg = background((S, S))
full = Image.alpha_composite(bg, glyph((S, S)))
down(full, 1024).save(out + '/icon.png')
down(rounded(full, int(S * 0.225)), 1024).save(out + '/logo.png')

# Android adaptive layers: 108dp canvas, art in the middle 66dp.
inner = int(S * 66 / 108)
art = glyph((inner, inner))
fg = Image.new('RGBA', (S, S), (0, 0, 0, 0))
fg.paste(art, ((S - inner) // 2, (S - inner) // 2), art)
down(fg, 432).save(out + '/android-foreground.png')
down(background((S, S)), 432).save(out + '/android-background.png')
mono = glyph((inner, inner), mono=True)
mfg = Image.new('RGBA', (S, S), (0, 0, 0, 0))
mfg.paste(mono, ((S - inner) // 2, (S - inner) // 2), mono)
down(mfg, 432).save(out + '/android-monochrome.png')
