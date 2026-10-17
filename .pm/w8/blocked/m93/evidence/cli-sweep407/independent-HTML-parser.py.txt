from html.parser import HTMLParser
import hashlib
def inspect_html(b):
 class Doc(HTMLParser):
  def __init__(self):super().__init__();self.stack=[];self.data={};self.links=[]
  def handle_starttag(self,tag,attrs):
   self.stack.append(tag)
   if tag=='a':self.links.append(dict(attrs).get('href'))
  def handle_endtag(self,tag):
   assert self.stack and self.stack[-1]==tag;self.stack.pop()
  def handle_data(self,s):
   if self.stack and s.strip():self.data.setdefault(self.stack[-1],[]).append(s)
 d=Doc();d.feed(b.decode('utf8'));d.close();assert not d.stack
 assert d.data['h1']==['Café 雪'] and d.data['strong']==['bold'] and d.data['em']==['emphasis'] and d.data['code']==['x < 7'] and d.data['li']==['alpha','beta'] and d.data['a']==['Guide'] and d.links==['https://example.invalid/guide?q=qa407']
 return {'bytes':len(b),'sha256':hashlib.sha256(b).hexdigest(),'elements':d.data,'links':d.links,'pass':True}
