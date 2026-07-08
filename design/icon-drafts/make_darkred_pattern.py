#!/usr/bin/env python3
"""
平铺补子纹样（深红色，比红底更深），形成同色系暗花层次。
"""
from PIL import Image as IM, ImageFilter as IF
import numpy as np
from make_icons import extract_silhouette, squircle_mask, YELLOW

S = 1024
sil = extract_silhouette()

# 1. 补子 tile：二值化纹样，填深红
g = np.array(IM.open('buzhi-src.jpg').convert('L'))
H, W = g.shape
fg = g < 200
binim = IM.fromarray((fg * 255).astype(np.uint8)).filter(IF.MedianFilter(size=3))
arr = np.array(binim) > 127

# 2. squircle 亮红渐变底
bg_mask = squircle_mask(S, n=5)
yy = np.arange(S).reshape(-1, 1)
c1 = np.array([200, 44, 44]); c2 = np.array([140, 24, 24])
t = (yy / S) * np.ones((1, S))
grad = np.zeros((S, S, 3), np.uint8)
for k in range(3):
    grad[..., k] = (c1[k] * (1 - t * 0.7) + c2[k] * (t * 0.7)).astype(np.uint8)
rgba = np.dstack([grad, (bg_mask * 255).astype(np.uint8)])
bg_master = IM.fromarray(rgba, 'RGBA')

ow, oh = W, H
# 参数：补子色（比红底暗）、大小、不透明度
variants = [
    ((130, 28, 28), 0.60, 0.60, 'lt1'),   # 较浅暗红 + 大
    ((110, 20, 20), 0.68, 0.60, 'lt2'),   # 中暗红 + 更大
    ((140, 34, 34), 0.55, 0.55, 'lt3'),   # 更浅 + 中大
]
for dark_red, ts, alpha, tag in variants:
    tile_rgba = np.zeros((H, W, 4), np.uint8)
    tile_rgba[arr] = (*dark_red, 255)
    tile = IM.fromarray(tile_rgba, 'RGBA').filter(IF.GaussianBlur(0.5))

    bg = bg_master.copy()
    nw, nh = int(ow * ts), int(oh * ts)
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

    # 人物剪影（金色顶层）
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
    bg.save(f'preview-appicon-darkred-{tag}.png')
    print('wrote', tag, 'tile', nw, 'x', nh)
