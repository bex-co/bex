import struct,zlib,hashlib
def decode_png(b):
 assert b[:8]==bytes.fromhex('89504e470d0a1a0a');pos=8;packed=b'';header=None;done=False
 while pos<len(b):
  n=int.from_bytes(b[pos:pos+4],'big');kind=b[pos+4:pos+8];data=b[pos+8:pos+8+n];crc=int.from_bytes(b[pos+8+n:pos+12+n],'big');assert zlib.crc32(kind+data)&0xffffffff==crc
  if kind==b'IHDR':header=struct.unpack('>IIBBBBB',data)
  if kind==b'IDAT':packed+=data
  pos+=n+12
  if kind==b'IEND':done=True;break
 assert done and pos==len(b) and header==(2,1,8,2,0,0,0)
 raw=zlib.decompress(packed);assert len(raw)==7 and raw[0] in range(5);rgb=[]
 for i,value in enumerate(raw[1:]):
  left=rgb[i-3] if i>=3 else 0;predictor=left if raw[0] in (1,4) else left//2 if raw[0]==3 else 0;rgb.append((value+predictor)&255)
 pixels=bytes(rgb);return {'width':2,'height':1,'filter':raw[0],'pixels_hex':pixels.hex(),'pixels_sha256':hashlib.sha256(pixels).hexdigest(),'PNG_sha256':hashlib.sha256(b).hexdigest()}
