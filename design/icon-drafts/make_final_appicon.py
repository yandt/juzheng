#!/usr/bin/env python3
"""
最终应用图标生成（深红.95 参数确定版）：
亮红渐变 squircle 底 + 深红补子平铺（高不透明度，小尺寸仍可见）+ 赭金人物剪影。
输出 build/appicon.png。
"""
from PIL import Image as IM, ImageFilter as IF
import numpy as np
import os
from make_icons import extract_silhouette, squircle_mask, YELLOW

S = 1024
HERE = os.path.dirname(os.path.abspath(__file__))
PROJECT = os.path.dirname(os.path.dirname(HERE))
OUT = os.path.join(PROJECT, 'build', 'appicon.png')
BUZHI = os.path.join(HERE, 'buzhi-src.jpg')

# 最终参数（深红.95）
DARK_RED = (120, 20, 20)   # 深红
TS = 0.55                  # 补子缩放
ALPHA = 0.95               # 高不透明度，小尺寸仍可见
PERSON_RATIO = 0.78

sil = extract_silhouette()

# 1. 补子 tile
g = np.array(IM.open(BUZHI).convert('L'))
H, W = g.shape
fg = g < 200
binim = IM.fromarray((fg * 255).astype(np.uint8)).filter(IF.MedianFilter(size=3))
arr = np.array(binim) > 127
tile_rgba = np.zeros((H, W, 4), np.uint8)
tile_rgba[arr] = (*DARK_RED, 255)
tile = IM.fromarray(tile_rgba, 'RGBA').filter(IF.GaussianBlur(0.5))

# 2. squircle 亮红渐变底
bg_mask = squircle_mask(S, n=5)
yy = np.arange(S).reshape(-1, 1)
c1 = np.array([200, 44, 44]); c2 = np.array([140, 24, 24])
t = (yy / S) * np.ones((1, S))
grad = np.zeros((S, S, 3), np.uint8
)
for k in range(3):
    grad[..., k] = (c1[k] * (1 - t * 0.7) + c2[k] * (t * 0.7)).astype(np.uint8)
rgba = np.dstack([grad, (bg_mask * 255).astype(np.uint8)])
bg = IM.fromarray(rgba, 'RGBA')

# 3. 平铺补子
nw, nh = int(W * TS), int(H * TS)
tile_s = tile.resize((nw, nh), IM.LANCZOS)
ta = np.array(tile_s).astype(np.float32)
ta[..., 3] *= ALPHA
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

# 4. 人物剪影（金色顶层）
ch, cw = sil.shape
target_h = int(S * PERSON_RATIO)
sc = target_h / ch
pw, ph = int(round(cw * sc)), target_h
small = IM.fromarray((sil * 255).astype(np.uint8)).resize((pw, ph), IM.LANCZOS)
m = np.array(small) > 127
person = np.zeros((ph, pw, 4), np.uint8)
person[m] = (*YELLOW, 255)
top = int((S - ph) / 2) - int(S * 0.03)
left = (S - pw) // 2
bg.alpha_composite(IM.fromarray(person, 'RGBA'), (left, top))

bg.save(OUT)
print('wrote', OUT, bg.size)
