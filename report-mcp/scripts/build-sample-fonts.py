#!/usr/bin/env python3
from pathlib import Path
from fontTools.ttLib import TTFont
from fontTools.varLib.instancer import instantiateVariableFont
from fontTools import subset
import shutil,hashlib,json,argparse,fontTools
if fontTools.__version__ != "4.68.0":
 raise SystemExit("Use fonttools==4.68.0 for reproducible subsets")
parser=argparse.ArgumentParser(description="Build the sample-compatible static Korean TrueType subsets")
parser.add_argument("--sources",type=Path,required=True,help="Noto VF TTFs and license files")
parser.add_argument("--dejavu",type=Path,required=True,help="DejaVu TTF directory")
parser.add_argument("--out",type=Path,default=Path(__file__).resolve().parents[1]/"pentestreport/assets/fonts")
args=parser.parse_args()
out=args.out
out.mkdir(parents=True,exist_ok=True)
raw=args.sources
expected_sources={'NotoSansCJKsc-VF.ttf':'990c807e79c25662a5a9ecf7f971baeb2bf2eab9a559e5ecf15cdfdb8561d21f','NotoSansMonoCJKsc-VF.ttf':'9fe57a71eb48e50cccb77903123d08857edefcf289c7b309462c4c3d82208126'}
for name,expected in expected_sources.items():
 if hashlib.sha256((raw/name).read_bytes()).hexdigest()!=expected:
  raise SystemExit('Noto input hash mismatch: '+name)
for name in ['DejaVuSans.ttf','DejaVuSans-Bold.ttf','DejaVuSansMono.ttf']:
 shutil.copyfile(args.dejavu/name,out/name)
for filename,weight,mono in [('NotoSansCJKsc-Regular.ttf',400,False),('NotoSansCJKsc-Bold.ttf',700,False),('NotoSansMonoCJKsc-Regular.ttf',400,True)]:
 f=TTFont(raw/('NotoSansMonoCJKsc-VF.ttf' if mono else 'NotoSansCJKsc-VF.ttf'))
 # Subset before instancing to keep memory bounded; Korean/Jamo and CJK punctuation.
 opts=subset.Options();opts.name_IDs=['*'];opts.name_legacy=True;opts.name_languages=['*'];opts.recalc_timestamp=False
 sub=subset.Subsetter(options=opts);sub.populate(unicodes=list(range(0x1100,0x1200))+list(range(0x2000,0x2070))+list(range(0x2460,0x2500))+list(range(0x3000,0x3300))+list(range(0xA960,0xA980))+list(range(0xAC00,0xD800))+list(range(0xFF00,0xFFF0)))
 sub.subset(f);f=instantiateVariableFont(f,{'wght':weight},inplace=True)
 family='Report Noto Sans Mono CJK SC' if mono else 'Report Noto Sans CJK SC';style='Bold' if weight==700 else 'Regular'
 for rec in f['name'].names:
  if rec.nameID in [1,2,4,6,16,17]:
   v={1:family,2:style,4:family+' '+style,6:"Report"+filename[:-4],16:family,17:style}[rec.nameID]
   rec.string=v.encode(rec.getEncoding())
 f.recalcTimestamp=False
 f.save(out/filename)
 print(filename,(out/filename).stat().st_size)
for name in ['OFL-NotoCJK.txt','LICENSE-DejaVu.txt']:shutil.copyfile(raw/name,out/name)
# Commit the manifest alongside the fonts; it is part of the template seal.
source_commit='f8d157532fbfaeda587e826d4cd5b21a49186f7c'
manifest={'fonttools_version':fontTools.__version__,'noto_source_commit':source_commit,'sources':{},'fonts':{}}
for name,rel in [('NotoSansCJKsc-VF.ttf','Sans/Variable/TTF/NotoSansCJKsc-VF.ttf'),('NotoSansMonoCJKsc-VF.ttf','Sans/Variable/TTF/Mono/NotoSansMonoCJKsc-VF.ttf')]:
 manifest['sources'][name]={'url':f'https://raw.githubusercontent.com/notofonts/noto-cjk/{source_commit}/{rel}','sha256':hashlib.sha256((raw/name).read_bytes()).hexdigest()}
for name in ['DejaVuSans.ttf','DejaVuSans-Bold.ttf','DejaVuSansMono.ttf','NotoSansCJKsc-Regular.ttf','NotoSansCJKsc-Bold.ttf','NotoSansMonoCJKsc-Regular.ttf']:
 f=TTFont(out/name)
 manifest['fonts'][name]={'sha256':hashlib.sha256((out/name).read_bytes()).hexdigest(),'revision':f['head'].fontRevision,'copyright':f['name'].getDebugName(0),'postscript_name':f['name'].getDebugName(6)}
for name in ['DejaVuSans.ttf','DejaVuSans-Bold.ttf','DejaVuSansMono.ttf']:
 manifest['sources'][name]={'upstream_release':'https://github.com/dejavu-fonts/dejavu-fonts/releases/tag/version_2_37','origin':'LibreOffice bundled DejaVu 2.37; upstream link is the release reference','input_sha256':hashlib.sha256((args.dejavu/name).read_bytes()).hexdigest()}
(out.parent/'font-provenance.json').write_text(json.dumps(manifest,ensure_ascii=False,indent=2)+'\n')
