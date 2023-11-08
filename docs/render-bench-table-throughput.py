import pandas as pd

def read_throughput_data(file_path):
    data = pd.read_csv(file_path)
    data['throughput'] = data['throughput'].str.replace(' MB/s', '').astype(int)
    return data['throughput'].mean()

json_file_path = 'out/throughput-json.csv'
cbor_file_path = 'out/throughput-cbor.csv'

json_avg_throughput = read_throughput_data(json_file_path)
cbor_avg_throughput = read_throughput_data(cbor_file_path)

print("| Serializer | Average Throughput |")
print("|-----------|--------------------|")
print(f"| JSON      | {round(json_avg_throughput)} MB/s |")
print(f"| CBOR      | {round(cbor_avg_throughput)} MB/s |")
