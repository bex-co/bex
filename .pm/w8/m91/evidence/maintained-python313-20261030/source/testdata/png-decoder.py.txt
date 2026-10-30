"""Independent bounded RGB8/noninterlaced PNG decode, including all filters."""
import hashlib,struct,zlib

def decode_png(blob):
 assert blob[:8]==b'\x89PNG\r\n\x1a\n';pos=8;packed=b'';header=None;done=False
 while pos<len(blob):
  length=int.from_bytes(blob[pos:pos+4],'big');kind=blob[pos+4:pos+8];data=blob[pos+8:pos+8+length];assert len(data)==length and pos+length+12<=len(blob);crc=int.from_bytes(blob[pos+8+length:pos+12+length],'big');assert zlib.crc32(kind+data)&0xffffffff==crc
  if kind==b'IHDR':assert header is None;header=struct.unpack('>IIBBBBB',data)
  if kind==b'IDAT':packed+=data
  pos+=length+12
  if kind==b'IEND':assert length==0;done=True;break
 assert done and pos==len(blob) and header is not None
 width,height,depth,color,compression,filters,interlace=header;assert 0<width<=1024 and 0<height<=1024 and (depth,color,compression,filters,interlace)==(8,2,0,0,0)
 stride=width*3;decompressor=zlib.decompressobj();raw=decompressor.decompress(packed,(stride+1)*height+1);assert len(raw)==(stride+1)*height and decompressor.eof and not decompressor.unused_data
 previous=bytes(stride);pixels=bytearray()
 for y in range(height):
  row=raw[y*(stride+1):(y+1)*(stride+1)];filter_kind=row[0];assert filter_kind in range(5);decoded=bytearray()
  for i,value in enumerate(row[1:]):
   left=decoded[i-3] if i>=3 else 0;up=previous[i];upper_left=previous[i-3] if i>=3 else 0
   if filter_kind==0:predictor=0
   elif filter_kind==1:predictor=left
   elif filter_kind==2:predictor=up
   elif filter_kind==3:predictor=(left+up)//2
   else:
    p=left+up-upper_left;a,b,c=abs(p-left),abs(p-up),abs(p-upper_left);predictor=left if a<=b and a<=c else up if b<=c else upper_left
   decoded.append((value+predictor)&255)
  pixels.extend(decoded);previous=decoded
 return {'width':width,'height':height,'pixels_hex':pixels.hex(),'pixels_sha256':hashlib.sha256(pixels).hexdigest(),'PNG_sha256':hashlib.sha256(blob).hexdigest()}
