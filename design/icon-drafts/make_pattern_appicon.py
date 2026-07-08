#!/usr/bin/env python3
"""
在 appicon 红底上平铺仙鹤补子纹样，再叠加金色人物剪影。
- 纹样 tile 用 crane-tile.svg 渲染后作为浅金色描线图案
- 平铺时旋转/错位，避免接缝过于规整
- 人物剪影在最上层
"""
import os, subprocess
from PIL import Image, ImageFilter
import numpy as np
from make_icons import extract_silhouette, squircle_mask, YELLOW

HERE = os.path.dirname(os.path.abspath(__file__))
PROJECT = os.path.dirname(os.path.dirname(HERE))

CHROME = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
S = 1024


def render_tile(tile_svg, out_png, scale_bg=(0, 0, 0, 0)):
    """渲染 svg tile 为 PNG。"""
    tmp = out_png + ".render"
    subprocess.run([CHROME, "--headless", "--disable-gpu", "--no-sandbox",
                    "--hide-scrollbars", "--window-size=400,480",
                    "--default-background-color=00000000",
                    f"--screenshot={tmp}",
                    f"file://{tile_svg}"], capture_output=True)
    im = Image.open(tmp).convert("RGBA")
    os.remove(tmp)
    return im


def make_appicon_with_pattern(sil, tile_png, out_path,
                              person_ratio=0.78,
                              tile_scale=0.5,
                              pattern_alpha=0.22):
    size = S
    # 1. squircle 红底渐变（与之前一致）
    bg_mask = squircle_mask(size, n=5)
    yy = np.arange(size).reshape(-1, 1)
    c1 = np.array([200, 44, 44]); c2 = np.array([140, 24, 24])
    t = (yy / size) * np.ones((1, size))
    grad = np.zeros((size, size, 3), np.uint8)
    for k in range(3):
        grad[..., k] = (c1[k] * (1 - t * 0.7) + c2[k] * (t * 0.7)).astype(np.uint8)
    rgba = np.dstack([grad, (bg_mask * 255).astype(np.uint8)])
    bg = Image.fromarray(rgba, "RGBA")

    # 2. 平铺纹样
    tile = Image.open(tile_png).convert("RGBA")
    tw, th = tile.size
    # 缩放 tile
    nw = int(tw * tile_scale); nh = int(th * tile_scale)
    tile_s = tile.resize((nw, nh), Image.LANCZOS)
    # 降低纹样不透明度，避免抢戏
    ta = np.array(tile_s).astype(np.float32)
    ta[..., 3] *= pattern_alpha
    # 加强纹样金色（让线条更明显）
    ta[..., 0] = np.clip(ta[..., 0] * 1.1, 0, 255)
    ta[..., 1] = np.clip(ta[..., 1] * 1.05, 0, 255)
    tile_s = Image.fromarray(ta.astype(np.uint8), "RGBA")

    pattern = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    # 错位平铺：奇数行偏移 nw/2
    rows = (size // nh) + 2
    cols = (size // nw) + 2
    for r in range(rows):
        offset_x = (nw // 2) if r % 2 else 0
        for c in range(cols):
            x = c * nw + offset_x - nw // 2
            y = r * nh - nh // 2
            pattern.alpha_composite(tile_s, (x, y))
    # 用 squircle mask 裁剪纹样到圆角方底内
    pmask = (bg_mask * 255).astype(np.uint8)
    parr = np.array(pattern)
    parr[..., 3] = (parr[..., 3].astype(np.float32) / 255 * pmask).astype(np.uint8)
    pattern = Image.fromarray(parr, "RGBA")
    bg.alpha_composite(pattern)

    # 3. 人物剪影（顶层）
    ch, cw = sil.shape
    target_h = int(size * person_ratio)
    scale = target_h / ch
    pw = int(round(cw * scale)); ph = target_h
    small = Image.fromarray((sil * 255).astype(np.uint8)).resize((pw, ph), Image.LANCZOS)
    m = np.array(small) > 127
    person_rgba = np.zeros((ph, pw, 4), np.uint8)
    person_rgba[m] = (*YELLOW, 255)
    top = int((size - ph) / 2) - int(size * 0.03)
    left = (size - pw) // 2
    bg.alpha_composite(Image.fromarray(person_rgba, "RGBA"), (left, top))

    bg.save(out_path)
    print("wrote", out_path, bg.size)


if __name__ == "__main__":
    sil = extract_silhouette()
    tile_png = os.path.join(HERE, "crane-tile.png")
    if not os.path.exists(tile_png):
        render_tile(os.path.join(HERE, "crane-tile.svg"), tile_png)
    make_appicon_with_pattern(sil, tile_png,
                              os.path.join(HERE, "preview-appicon-pattern.png"),
                              person_ratio=0.78, tile_scale=0.5, pattern_alpha=0.22)
