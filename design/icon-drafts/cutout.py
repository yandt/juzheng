#!/usr/bin/env python3
"""
按用户思路：参考图去背景(浅黄)，人物填成单色黄。
- 亮度阈值抠出人物前景
- flood-fill 补全内部空洞（服饰亮色、五官等被前景包围的区域都算人物）
- 高斯平滑外轮廓
- 输出透明背景黄色人物 + 三套底色圆合成版
"""
import os
from PIL import Image, ImageFilter
import numpy as np
from collections import deque

HERE = os.path.dirname(os.path.abspath(__file__))
SRC = os.path.join(HERE, "reference.jpg")
YELLOW = (230, 188, 79)  # 赭金黄色

# 1. 灰度 + 二值化（人物暗 → 前景）
g = np.array(Image.open(SRC).convert("L"))
fg = g < 120

# 2. flood-fill 背景（从四边出发），未被访问 = 完整人物(含内部空洞)
H, W = fg.shape
visited = np.zeros_like(fg, dtype=bool)
q = deque()
def seed(i, j):
    if not fg[i, j] and not visited[i, j]:
        visited[i, j] = True
        q.append((i, j))
for i in range(H):
    seed(i, 0); seed(i, W - 1)
for j in range(W):
    seed(0, j); seed(H - 1, j)
while q:
    i, j = q.popleft()
    for di, dj in ((-1, 0), (1, 0), (0, -1), (0, 1)):
        ni, nj = i + di, j + dj
        if 0 <= ni < H and 0 <= nj < W and not fg[ni, nj] and not visited[ni, nj]:
            visited[ni, nj] = True
            q.append((ni, nj))
person = ~visited  # 完整人物剪影

# 3. 平滑外轮廓（高斯模糊再二值化）
sm = Image.fromarray((person * 255).astype(np.uint8)).filter(ImageFilter.GaussianBlur(2.5))
person_sm = np.array(sm) > 127

# 4. 裁剪到人物 bbox
ys, xs = np.where(person_sm)
pad = 6
x0, x1 = max(xs.min() - pad, 0), min(xs.max() + pad, W)
y0, y1 = max(ys.min() - pad, 0), min(ys.max() + pad, H)
crop = person_sm[y0:y1, x0:x1]

# 5. 放到 1024 画布（人物高度≈640，垂直略上移留帽顶空间）
ch, cw = crop.shape
TARGET_H = 660
scale = TARGET_H / ch
new_w = int(round(cw * scale))
small = Image.fromarray((crop * 255).astype(np.uint8)).resize((new_w, TARGET_H), Image.LANCZOS)
canvas = 1024
out = Image.new("RGBA", (canvas, canvas), (0, 0, 0, 0))
# 垂直略上移：帽顶距顶 ~180
top = 180
left = (canvas - new_w) // 2
# 把 mask 转成黄色
rgba = np.zeros((TARGET_H, new_w, 4), dtype=np.uint8)
m = np.array(small) > 127
rgba[m] = (*YELLOW, 255)
yellow_person = Image.fromarray(rgba, "RGBA")
out.paste(yellow_person, (left, top), yellow_person)
out.save(os.path.join(HERE, "juzheng-cutout-bare.png"))
print("wrote bare cutout", out.size)

# 6. 三套底色圆合成
variants = [
    ("a", (168, 38, 38), (122, 24, 24), (92, 16, 16)),
    ("b", (36, 58, 102), (21, 36, 68), (14, 26, 51)),
    ("c", (34, 34, 34), (14, 14, 14), (0, 0, 0)),
]
for v, c1, c2, stroke in variants:
    bg = Image.new("RGBA", (canvas, canvas), (0, 0, 0, 0))
    # 径向渐变圆
    yy, xx = np.mgrid[0:canvas, 0:canvas]
    dx, dy = xx - canvas / 2, yy - canvas / 2
    r = np.sqrt(dx * dx + dy * dy)
    R = 492
    t = np.clip(r / R, 0, 1)
    grad = np.zeros((canvas, canvas, 3), dtype=np.uint8)
    for k in range(3):
        grad[..., k] = (c1[k] * (1 - t) + c2[k] * t).astype(np.uint8)
    mask_circle = r <= R
    rgba_bg = np.dstack([grad, (mask_circle * 255).astype(np.uint8)])
    bg = Image.fromarray(rgba_bg, "RGBA")
    bg.alpha_composite(out)
    bg.save(os.path.join(HERE, f"juzheng-icon-{v}.png"))
    print("wrote", v, bg.size)
