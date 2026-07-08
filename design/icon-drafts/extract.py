#!/usr/bin/env python3
"""
从参考图提取明朝文官轮廓 → 平滑 → potrace 描摹 → 套底色圆 → 输出 SVG。
策略：
  1. 参考图背景为浅黄/米色，人物为深色 → 按亮度阈值二值化（人物=黑/前景）
  2. 适当膨胀/平滑轮廓，去掉服饰花纹干扰，只保留外轮廓填充
  3. 用 potrace 描摹为平滑 SVG path
  4. 把 path 居中缩放到 1024 画布，套底色圆
"""
import subprocess, sys, os, tempfile
from PIL import Image, ImageFilter, ImageOps
import numpy as np

HERE = os.path.dirname(os.path.abspath(__file__))
SRC = os.path.join(HERE, "reference.jpg")

# --- 1. 预处理：转灰度、二值化（人物=前景黑） ---
im = Image.open(SRC).convert("L")
# 参考图背景偏亮(>150)，人物偏暗。阈值取 120，人物=黑
g = np.asarray(im)
mask = (g < 120).astype(np.uint8) * 255  # 人物=255(白)，背景=0(黑)
binim = Image.fromarray(mask)

# --- 2. 形态学平滑：先膨胀合并服饰花纹缝隙，再模糊，再二值化，得到干净外轮廓 ---
# 中值滤波去噪
binim = binim.filter(ImageFilter.MedianFilter(size=5))
# 膨胀（合并内部缝隙，让人物成为实心块）
arr = np.asarray(binim)
# 多次膨胀
from PIL import ImageFilter
for _ in range(3):
    arr = np.asarray(Image.fromarray(arr).filter(ImageFilter.MaxFilter(size=5)))
# 高斯模糊使外轮廓圆润
sm = Image.fromarray(arr).filter(ImageFilter.GaussianBlur(radius=6))
arr = (np.asarray(sm) > 127).astype(np.uint8) * 255
# 再做一次平滑
final = Image.fromarray(arr).filter(ImageFilter.GaussianBlur(radius=2))

# 裁剪到人物包围盒，加白边
arr = np.asarray(final)
ys, xs = np.where(arr > 127)
if len(xs) == 0:
    print("ERROR: 未检测到人物", file=sys.stderr); sys.exit(1)
pad = 20
x0, x1 = max(xs.min()-pad,0), min(xs.max()+pad, arr.shape[1])
y0, y1 = max(ys.min()-pad,0), min(ys.max()+pad, arr.shape[0])
cropped = Image.fromarray(arr).crop((x0,y0,x1,y1))
# 转为 potrace 输入：前景=黑(人物)
# potrace: 黑色像素被视为需要描摹的形状
pnm = ImageOps.invert(cropped)  # 反转：人物=黑
buf = pnm.convert("1")

tmp = tempfile.mkdtemp()
bmp = os.path.join(tmp, "in.bmp")
out_prefix = os.path.join(tmp, "out.svg")
buf.save(bmp)

# --- 3. potrace 描摹 ---
subprocess.run(["potrace", bmp, "-s", "-o", out_prefix,
                "--alphamax","1.4","--turdsize","20","--opttolerance","0.5"],
               check=True, capture_output=True)
svg = out_prefix
with open(svg) as f:
    s = f.read()
# 提取 path d
import re
m = re.search(r'<path[^>]*d="([^"]+)"', s)
if not m:
    print("ERROR: potrace 未生成 path", file=sys.stderr); sys.exit(1)
path_d = m.group(1)

# --- 4. 计算路径包围盒以缩放居中（potrace 用原始像素坐标） ---
# potrace 路径坐标基于输入位图像素
nums = [float(x) for x in re.findall(r'-?\d+\.?\d*', path_d)]
xs2 = nums[0::2]; ys2 = nums[1::2]
pw, ph = max(xs2)-min(xs2), max(ys2)-min(ys2)

# 目标：在 1024 画布中，人物高度约占 600px，垂直略上移居中
TARGET_H = 640
scale = TARGET_H / ph
new_w = pw * scale
# 在 1024 画布居中（略上移给帽子留空间）
tx = (1024 - new_w)/2 - min(xs2)*scale
ty = 200 - min(ys2)*scale  # 顶部留 200px

# 重新生成路径数值（对每个数值乘 scale 再加偏移）
def repl(mm):
    v = float(mm.group(0))
    return f"{v:.2f}"
# 分离 x/y 难以纯正则做，改为：构造变换，用 <g transform> 包裹更简单
# 直接用 transform 缩放平移整条 path
g_transform = f"translate({tx:.2f} {ty:.2f}) scale({scale:.4f})"

# --- 5. 生成三套方案 SVG ---
variants = [
    ("a", "A82626", "7A1818", "5C1010", "E8C879", "朱明红底+赭金轮廓"),
    ("b", "243A66", "152444", "0E1A33", "EDE6D2", "藏青底+米白轮廓"),
    ("c", "222222", "0E0E0E", "000000", "D9B45A", "墨黑底+赤金轮廓"),
]
for v, c1, c2, stroke, fg, desc in variants:
    svg_out = f'''<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024" width="1024" height="1024">
  <title>居正 · 明代文官轮廓图标（参考图描摹）{desc}</title>
  <defs>
    <radialGradient id="bg{v}" cx="50%" cy="40%" r="70%">
      <stop offset="0%" stop-color="#{c1}"/>
      <stop offset="100%" stop-color="#{c2}"/>
    </radialGradient>
  </defs>
  <circle cx="512" cy="512" r="492" fill="url(#bg{v})"/>
  <circle cx="512" cy="512" r="492" fill="none" stroke="#{stroke}" stroke-width="6" opacity="0.5"/>
  <g fill="#{fg}" transform="{g_transform}">
    <path d="{path_d}"/>
  </g>
</svg>
'''
    out = os.path.join(HERE, f"juzheng-icon-{v}.svg")
    with open(out, "w") as f:
        f.write(svg_out)
    print("wrote", out)
print("done. path bbox pw,ph=", round(pw,1), round(ph,1), "scale=", round(scale,4))
