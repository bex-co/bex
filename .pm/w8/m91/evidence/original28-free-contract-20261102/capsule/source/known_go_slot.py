"""Tiny offset scanner for ONE known top-level Go var; not a general Go parser.

Comments/quoted/raw/rune literals are opaque. Only the existing canonical zero,
plain inferred-string and plain typed-string declarations are supported.
"""
import json,re
import core as c

NAME=b'nativeCommonBase'
REF=re.compile(r'ghcr\.io/[a-z0-9._/-]+@sha256:[a-f0-9]{64}\Z')

def word(byte):return byte==95 or 65<=byte<=90 or 97<=byte<=122 or byte>=128

def tokens(source):
    c.require(type(source) is bytes,'known Go slot source must be bytes')
    result=[];i=0;braces=parens=brackets=0;n=len(source)
    while i<n:
        byte=source[i]
        if byte in b' \t\r\n':i+=1;continue
        if source[i:i+2]==b'//':
            end=source.find(b'\n',i+2);i=n if end<0 else end+1;continue
        if source[i:i+2]==b'/*':
            end=source.find(b'*/',i+2);c.require(end>=0,'unterminated Go block comment');i=end+2;continue
        start=i;kind='symbol'
        if byte in (34,39,96):
            quote=byte;kind={34:'string',39:'rune',96:'raw'}[byte];i+=1
            while i<n:
                if source[i]==quote:i+=1;break
                if quote!=96:
                    c.require(source[i] not in (10,13),'newline in quoted Go literal')
                    if source[i]==92:
                        c.require(i+1<n,'unterminated Go escape');i+=2;continue
                i+=1
            else:raise c.Unproved('unterminated Go literal')
        elif word(byte):
            kind='identifier';i+=1
            while i<n and (word(source[i]) or 48<=source[i]<=57):i+=1
        else:i+=1
        text=source[start:i];result.append((kind,text,start,i,braces,parens,brackets))
        if kind=='symbol':
            if text==b'{':braces+=1
            elif text==b'}':braces-=1
            elif text==b'(':parens+=1
            elif text==b')':parens-=1
            elif text==b'[':brackets+=1
            elif text==b']':brackets-=1
            c.require(min(braces,parens,brackets)>=0,'unbalanced Go delimiter')
    c.require((braces,parens,brackets)==(0,0,0),'unbalanced Go source')
    return result

def declaration(source):
    items=tokens(source);found=[]
    for index,item in enumerate(items):
        kind,text,start,end,braces,parens,brackets=item
        if kind!='identifier' or text!=b'var' or (braces,parens,brackets)!=(0,0,0):continue
        following=items[index+1:]
        c.require(following,'incomplete top-level Go var')
        if following[0][1]==b'(':
            # Grouped declarations are deliberately outside this known slot's
            # canonical forms; never count a comment/string within one as var.
            for token in following[1:]:
                if token[1]==b')' and token[5]==1:break
                c.require(not (token[0]=='identifier' and token[1]==NAME and token[4]==0),
                          'grouped known Go slot unsupported')
            continue
        if following[0][0]=='identifier' and following[0][1]==NAME:
            found.append(item)
        else:
            # Catch a second name in a top-level var name-list, not a value
            # expression that happens to read the known global.
            offset=0
            while offset+2<len(following) and following[offset+1][1]==b',':
                c.require(following[offset+2][1]!=NAME,'multi-name known Go slot unsupported');offset+=2
    c.require(len(found)==1,'unique actual top-level nativeCommonBase declaration required')
    start=found[0][2]
    c.require(start==0 or source[start-1:start]==b'\n','known Go var is not a canonical top-level line')
    end=source.find(b'\n',start);c.require(end>=0,'known Go declaration requires canonical final LF');end+=1
    body=source[start:end]
    if body==b'var nativeCommonBase string\n':return {'form':'zero','value':None,'start':start,'end':end}
    for form,prefix in [('inferred',b'var nativeCommonBase = '),('typed',b'var nativeCommonBase string = ')]:
        if body.startswith(prefix):
            raw=body[len(prefix):-1]
            try:value=json.loads(raw)
            except (ValueError,UnicodeError):raise c.Unproved('known Go slot is not a canonical quoted immutable REF') from None
            c.require(isinstance(value,str) and REF.fullmatch(value) and raw==json.dumps(value).encode(),
                      'known Go slot is not a canonical quoted immutable REF')
            return {'form':form,'value':value,'start':start,'end':end}
    raise c.Unproved('unsupported actual top-level nativeCommonBase declaration')
