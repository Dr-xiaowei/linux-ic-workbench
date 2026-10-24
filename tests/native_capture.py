#!/usr/bin/env python
"""Capture the isolated X11 fixture using libX11; Python 2.6 compatible."""
import ctypes as C, struct, zlib, sys
class XImage(C.Structure):
 _fields_=[('width',C.c_int),('height',C.c_int),('xoffset',C.c_int),('format',C.c_int),('data',C.c_void_p),('byte_order',C.c_int),('bitmap_unit',C.c_int),('bitmap_bit_order',C.c_int),('bitmap_pad',C.c_int),('depth',C.c_int),('bytes_per_line',C.c_int),('bits_per_pixel',C.c_int),('red_mask',C.c_ulong),('green_mask',C.c_ulong),('blue_mask',C.c_ulong)]
def capture(path,width=1360,height=920):
 x=C.CDLL('libX11.so.6');x.XOpenDisplay.restype=C.c_void_p;x.XDefaultRootWindow.argtypes=[C.c_void_p];x.XDefaultRootWindow.restype=C.c_ulong
 x.XGetImage.argtypes=[C.c_void_p,C.c_ulong,C.c_int,C.c_int,C.c_uint,C.c_uint,C.c_ulong,C.c_int];x.XGetImage.restype=C.POINTER(XImage)
 x.XDestroyImage.argtypes=[C.POINTER(XImage)];x.XCloseDisplay.argtypes=[C.c_void_p]
 d=x.XOpenDisplay(None)
 if not d:raise RuntimeError('X display unavailable')
 img=x.XGetImage(d,x.XDefaultRootWindow(d),0,0,width,height,C.c_ulong(-1).value,2)
 if not img:raise RuntimeError('X capture unavailable')
 try:
  v=img.contents;assert v.bits_per_pixel==32 and v.byte_order==0 and v.red_mask==0xff0000
  rows=[]
  for row in range(height):
   b=bytearray(C.string_at(v.data+row*v.bytes_per_line,width*4));rgb=bytearray(width*3)
   rgb[0::3]=b[2::4];rgb[1::3]=b[1::4];rgb[2::3]=b[0::4];rows.append(b'\0'+bytes(rgb))
  def chunk(kind,data):return struct.pack('>I',len(data))+kind+data+struct.pack('>I',zlib.crc32(kind+data)&0xffffffff)
  data=b'\x89PNG\r\n\x1a\n'+chunk(b'IHDR',struct.pack('>IIBBBBB',width,height,8,2,0,0,0))+chunk(b'IDAT',zlib.compress(b''.join(rows),6))+chunk(b'IEND',b'')
  with open(path,'wb') as out:out.write(data)
 finally:x.XDestroyImage(img);x.XCloseDisplay(d)
if __name__=='__main__':capture(sys.argv[1])
