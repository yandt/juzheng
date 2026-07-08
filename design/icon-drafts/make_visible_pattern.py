#!/usr/bin/env python3
"""
试几档补子配色/不透明度，并同步生成 64px 预览（模拟 Dock 小尺寸），
确保小尺寸下仙鹤仍可见。
"""
from PIL import Image as IM, ImageFilter as IF
import numpy as np
from make_icons import extract_silhouette, squircle_mask, YELLOW

S = 1024
sil = extract_silhouette()

g = np.array(IM.open('buzhi-src.jpg').convert('L'))
H, W = g.shape
fg = g < 200
binim = IM.fromarray((fg * 255).astype(np.uint8)).filter(IF.MedianFilter(size=3))
arr = np.array(binim) > 127

bg_mask = squircle_mask(S, n=5)
yy = np.arange(S).reshape(-1, 1)
c1 = np.array([200, 44, 44]); c2 = np.array([140, 24, 24])
t = (yy / S) * np.ones((1, S))
grad = np.zeros((S, S, 3), np.uint8)
for k in range(3):
    grad[..., k] = (c1[k] * (1 - t * 0.7) + c2[k] * (t * 0.7)).astype(np.uint8)
rgba = np.dstack([grad, (bg_mask * 255).astype(np.uint8)])
bg_master = IM.fromarray(rgba, 'RGBA')

# 候选：(补子色, 不透明度, tag)
# 金色系——高对比，小尺寸最清晰
# 亮红——比暗红明显但仍同色系
variants = [
    ((230, 188, 79), 0.85, 'gold-strong'),    # 金色高不透明
    ((245, 210, 110), 0.75, 'gold-bright'),   # 亮金
    ((255, 230, 150), 0.65, 'gold-pale'),     # 浅金更柔
    ((120, 20, 20), 0.95, 'reddark-strong'),  # 深红但近全不透明
]

for color, alpha, tag in variants:
    tile_rgba = np.zeros((H, W, 4), np.uint8)
    tile_rgba[arr] = (*color, 255)
    tile = IM.fromarray(tile_rgba, 'RGBA').filter(IF.GaussianBlur(0.5))

    bg = bg_master.copy()
    TS = 0.55
    nw, nh = int(W * TS), int(H * TS)
    tile_s = tile.resize((nw, nh), IM.LANCZOS)
    ta = np.array(tile_s).astype(np.float32)
    ta[..., 3] *= alpha
    tile_s = IM.fromarray(ta.astype(np.uint8), 'RGBA')
    pattern = IM.new('RGBA', (S, S), (0, 0, 0, 0))
    rows = (S // nh) + 2
    cols = (S // nw) + 2
    for r in range(rows):
        offx = (nw // 2) if r % 2 else 0
        for c in range(cols):
            x = c * nw + offx - nw // 2
            y = r * nh - nh // 2
            pattern.alpha_composite(tile_s, (x, y))
    parr = np.array(pattern)
    parr[..., 3] = (parr[..., 3].astype(np.float32) / 255 * (bg_mask * 255).astype(np.uint8)).astype(np.uint8)
    bg.alpha_composite(IM.fromarray(parr, 'RGBA'))

    ch, cw = sil.shape
    target_h = int(S * 0.78)
    sc = target_h / ch
    pw, ph = int(round(cw * sc)), target_h
    small = IM.fromarray((sil * 255).astype(np.uint8)).resize((pw, ph), IM.LANCZOS)
    m = np.array(small) > 127
    person = np.zeros((ph, pw, 4), np.uint8)
    person[m] = (*YELLOW, 255)
    top = int((S - ph) / 2) - int(S * 0.03)
    left = (S - pw) // 2
    bg.alpha_composite(IM.fromarray(person, 'RGBA'), (left, top))
    bg.save(f'preview-appicon-{tag}.png')
    # 同步生成 64px 预览（模拟 Dock 小尺寸）
    bg.resize((64, 64), IM.LANCZOS).save(f'preview-appicon-{tag}-64.png')
    print('wrote', tag)
