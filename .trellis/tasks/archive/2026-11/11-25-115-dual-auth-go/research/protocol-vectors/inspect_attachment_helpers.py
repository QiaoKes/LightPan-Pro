import marshal,types,pathlib,dis,io,json
V=pathlib.Path('/Users/saki/Desktop/code/ebn/restored-python/vendor/p115client');O=pathlib.Path(__file__).resolve().parent
names={'get_attr','iterdir_raw','fs_files_iter','fs_files_iter_raw','fs_files','get_proapi_origin','get_webapi_origin','fs_folder_app','iter_file_list','iter_fs_files','iter_page','_get_pickcode','get_ancestors','download_files_app','download_folders_app','fs_files_app','iter_download_files','iter_download_nodes','iter_download_nodes_raw','iter_download_dirs','iter_download_dirs_raw','iter_files_with_path_skim','iter_files_with_path','ensure_attr_path','iter_files','iterdir','get_id_to_path','fs_file_skim','fs_files_app','fs_files','fs_video_subtitle','iter_files_raw','iterdir_raw','get_path_to_cid','iter_nodes_using_event','traverse_files'}
def walk(c):
 yield c
 for v in c.co_consts:
  if isinstance(v,types.CodeType):yield from walk(v)
for rel in ['tool/iterdir','tool/fs_files','tool/attr','tool/download','client']:
 c=marshal.loads((V/(rel+'.pyc')).read_bytes()[16:]);buf=io.StringIO()
 for x in walk(c):
  if any(p in names for p in x.co_qualname.split('.')):
   print('QUALNAME',x.co_qualname,'SOURCE',x.co_firstlineno,file=buf);dis.dis(x,file=buf,depth=0)
 (O/('attachments-'+rel.replace('/','-')+'.dis.txt')).write_text(buf.getvalue())
