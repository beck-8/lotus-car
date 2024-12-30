import os
import hashlib
import json
from concurrent.futures import ThreadPoolExecutor
from tqdm import tqdm
from datetime import datetime

def load_existing_md5(output_file):
    """
    加载已存在的MD5记录
    如果文件不存在则返回空字典
    """
    try:
        with open(output_file, 'r', encoding='utf-8') as f:
            return json.load(f)
    except (FileNotFoundError, json.JSONDecodeError):
        return {}

def save_single_md5(md5_dict, output_file, rel_path, md5_value):
    """
    增量保存单个文件的MD5值
    """
    md5_dict[rel_path] = md5_value
    with open(output_file, 'w', encoding='utf-8') as f:
        json.dump(md5_dict, f, ensure_ascii=False, indent=4)

def calculate_md5(file_path, chunk_size=8192):
    """
    计算单个文件的MD5值
    使用分块读取来处理大文件
    """
    md5_hash = hashlib.md5()
    try:
        with open(file_path, 'rb') as f:
            while chunk := f.read(chunk_size):
                md5_hash.update(chunk)
        return md5_hash.hexdigest()
    except Exception as e:
        print(f"计算文件 {file_path} 的MD5时出错: {str(e)}")
        return None

def generate_md5_list(directory, output_file):
    """
    生成目录下所有文件的MD5值
    增量计算并保存，跳过已处理的文件
    """
    # 加载已存在的MD5记录
    md5_dict = load_existing_md5(output_file)
    files = []
    
    # 收集所有文件路径
    print("正在扫描目录...")
    for root, _, filenames in os.walk(directory):
        for filename in filenames:
            file_path = os.path.join(root, filename)
            rel_path = os.path.relpath(file_path, directory)
            
            # 检查文件是否已处理
            if rel_path in md5_dict:
                continue
                
            files.append((rel_path, file_path))
    
    if not files:
        print("没有新的文件需要处理")
        return md5_dict
    
    print(f"发现 {len(files)} 个新文件需要处理")
    
    # 使用线程池计算MD5
    with ThreadPoolExecutor() as executor:
        for rel_path, file_path in tqdm(files, desc="计算MD5"):
            # 计算MD5
            md5 = calculate_md5(file_path)
            if md5:
                # 立即保存结果
                save_single_md5(md5_dict, output_file, rel_path, md5)
    
    return md5_dict

def save_verification_results(mismatches, missing, output_file):
    """
    将验证结果保存到文本文件
    """
    timestamp = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    
    with open(output_file, 'w', encoding='utf-8') as f:
        f.write(f"MD5校验结果报告\n")
        f.write(f"生成时间: {timestamp}\n")
        f.write("-" * 50 + "\n\n")
        
        if not mismatches and not missing:
            f.write("验证结果: 所有文件MD5匹配！\n")
            return
        
        if mismatches:
            f.write("MD5不匹配的文件:\n")
            for file in mismatches:
                f.write(f"- {file}\n")
            f.write("\n")
            
        if missing:
            f.write("缺失的文件:\n")
            for file in missing:
                f.write(f"- {file}\n")
            f.write("\n")
        
        # 写入统计信息
        total_issues = len(mismatches) + len(missing)
        f.write(f"\n统计信息:\n")
        f.write(f"- MD5不匹配的文件数: {len(mismatches)}\n")
        f.write(f"- 缺失的文件数: {len(missing)}\n")
        f.write(f"- 问题文件总数: {total_issues}\n")

def compare_md5_lists(server_md5_file, local_directory):
    """
    比较服务器和本地的MD5值
    """
    # 读取服务器端的MD5列表
    with open(server_md5_file, 'r', encoding='utf-8') as f:
        server_md5_dict = json.load(f)
    
    # 计算本地文件的MD5
    local_md5_file = "local_md5.json"  # 本地MD5结果也保存，避免重复计算
    local_md5_dict = generate_md5_list(local_directory, local_md5_file)
    
    # 比较结果
    mismatches = []
    missing = []
    
    for rel_path, server_md5 in server_md5_dict.items():
        if rel_path not in local_md5_dict:
            missing.append(rel_path)
        elif local_md5_dict[rel_path] != server_md5:
            mismatches.append(rel_path)
    
    return mismatches, missing

if __name__ == "__main__":
    import argparse
    
    parser = argparse.ArgumentParser(description='MD5校验工具')
    parser.add_argument('--mode', choices=['generate', 'verify'], required=True,
                       help='运行模式：generate(生成MD5列表) 或 verify(验证MD5)')
    parser.add_argument('--directory', required=True,
                       help='要处理的目录路径')
    parser.add_argument('--output', default='md5_list.json',
                       help='MD5列表的输出文件路径（生成模式）或服务器MD5列表文件路径（验证模式）')
    parser.add_argument('--report', default='verification_report.txt',
                       help='验证结果报告的输出文件路径（仅验证模式）')
    
    args = parser.parse_args()
    
    if args.mode == 'generate':
        print(f"正在处理目录 {args.directory} 下的文件...")
        md5_dict = generate_md5_list(args.directory, args.output)
        print(f"已完成所有文件的MD5计算，结果保存在 {args.output}")
        print(f"总共处理了 {len(md5_dict)} 个文件")
    
    else:  # verify mode
        print(f"正在验证目录 {args.directory} 下的文件...")
        mismatches, missing = compare_md5_lists(args.output, args.directory)
        
        # 保存验证结果到报告文件
        save_verification_results(mismatches, missing, args.report)
        print(f"验证报告已保存到 {args.report}")
        
        # 在控制台显示简要结果
        if not mismatches and not missing:
            print("验证完成：所有文件MD5匹配！")
        else:
            total_issues = len(mismatches) + len(missing)
            print(f"\n发现 {total_issues} 个问题：")
            print(f"- {len(mismatches)} 个文件MD5不匹配")
            print(f"- {len(missing)} 个文件缺失")
            print(f"详细信息请查看报告文件：{args.report}")

# 在服务器端生成MD5列表
# python md5_check.py --mode generate --directory /path/to/files --output md5_list.json

# 在本地验证文件
# python md5_check.py --mode verify --directory /path/to/local/files --output md5_list.json --report verification_report.txt