#!/usr/bin/env python3
"""
处理用户提供的补子图：
1. 去白底（提取深色纹样为前景）
2. 转单色金色描线（保留纹样细节，去掉灰度填充的"冲淡"问题）
3. potrace 转矢量 SVG，便于无损缩放
输出：buzhi-vector.svg + buzhi-overlay.png（金色透明纹样）
"""
import os, subprocess, tempfile
from PIL import Image, ImageFilter
import numpy as np
import re

HERE = os.path.dirname(os.path.abspath(__file__))
SRC = os.path.join(HERE, "buzhi-src.jpg")
GOLD = (230, 188, 79)  # 与人物剪影一致的赭金

g = np.array(Image.open(SRC).convert("L"))
H, W = g.shape

# 1. 二值化：白色背景(>200)为透明，深色纹样(<200)为前景
fg = g < 200

# 2. 形态学清理：开运算去噪点
from PIL import ImageFilter as IF
binim = Image.fromarray((fg * 255).astype(np.uint8))
binim = binim.filter(IF.MedianFilter(size=3))
arr = np.array(binim) > 127

# 3. 直接做彩色叠加图（金色透明纹样），供平铺使用
rgba = np.zeros((H, W, 4), np.uint8)
rgba[arr] = (*GOLD, 255)
overlay = Image.fromarray(rgba, "RGBA")
# 轻微模糊边缘，避免锯齿
overlay = overlay.filter(IF.GaussianBlur(0.6))
overlay.save(os.path.join(HERE, "buzhi-overlay.png"))
print("wrote buzhi-overlay.png", overlay.size)

# 4. potrace 转矢量
tmp = tempfile.mkdtemp()
bmp = os.path.join(tmp, "in.bmp")
Image.fromarray((arr * 255).astype(np.uint8)).save(bmp)
out_svg = os.path.join(tmp, "out.svg")
subprocess.run(["potrace", bmp, "-s", "-o", out_svg,
                "--alphamax", "1.0", "--turdsize", "8", "--opttolerance", "0.4"],
               check=True, capture_output=True)
svg = open(out_svg).read()
m = re.search(r'<path[^>]*d="([^"]+)"', svg)
path_d = m.group(1)
# 包装成完整 svg，viewBox 用原图尺寸
final_svg = f'''<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} {H}" width="{W}" height="{H}">
  <path d="{path_d}" fill="#{GOLD[0]:02x}{GOLD[1]:02x}{GOLD[2]:02x}"/>
</svg>
'''
with open(os.path.join(HERE, "buzhi-vector.svg"), "w") as f:
    f.write(final_svg)
print("wrote buzhi-vector.svg")
