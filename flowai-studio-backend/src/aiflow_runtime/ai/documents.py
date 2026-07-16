from __future__ import annotations
import io
from pathlib import Path
from xml.etree import ElementTree
from zipfile import ZipFile
from pypdf import PdfReader

def parse_document(data:bytes,filename:str,mime_type:str)->str:
    suffix=Path(filename).suffix.lower()
    if suffix in {".txt",".md",".markdown"} or mime_type.startswith("text/"):return data.decode("utf-8",errors="replace")
    if suffix==".pdf" or mime_type=="application/pdf":return "\n".join(page.extract_text() or "" for page in PdfReader(io.BytesIO(data)).pages)
    if suffix==".docx" or "wordprocessingml" in mime_type:
        with ZipFile(io.BytesIO(data)) as archive:
            root=ElementTree.fromstring(archive.read("word/document.xml"))
        namespace="{http://schemas.openxmlformats.org/wordprocessingml/2006/main}"
        return "\n".join("".join(node.text or "" for node in paragraph.iter(f"{namespace}t")) for paragraph in root.iter(f"{namespace}p"))
    raise ValueError("Unsupported document type")

def chunks(text:str,size:int,overlap:int)->list[str]:
    clean=text.strip()
    if not clean:return []
    result=[];start=0;step=max(1,size-min(overlap,size-1))
    while start<len(clean):result.append(clean[start:start+size]);start+=step
    return result
