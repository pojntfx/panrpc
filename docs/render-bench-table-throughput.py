import pandas as pd

def read_throughput_data(file_path, serializer):
    data = pd.read_csv(file_path)
    data['throughput'] = data['throughput'].str.replace(' MB/s', '').astype(int)
    avg_throughput = data.groupby('language')['throughput'].mean().reset_index()
    avg_throughput['serializer'] = serializer
    return avg_throughput

json_file_path = 'out/throughput-json.csv'
cbor_file_path = 'out/throughput-cbor.csv'

json_avg_throughput = read_throughput_data(json_file_path, 'JSON')
cbor_avg_throughput = read_throughput_data(cbor_file_path, 'CBOR')

combined_avg_throughput = pd.concat([json_avg_throughput, cbor_avg_throughput])
combined_avg_throughput = combined_avg_throughput.sort_values(by=['language', 'serializer'])

print("| Serializer | Average Throughput |")
print("|------------|--------------------|")

for index, row in combined_avg_throughput.iterrows():
    print(f"| {row['serializer']} ({row['language']}) | {round(row['throughput'])} MB/s |")
