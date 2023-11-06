import pandas as pd

def extract_requests(s):
    return int(s.split()[0])

def process_data(json_path, cbor_path):
    json_data = pd.read_csv(json_path)
    cbor_data = pd.read_csv(cbor_path)

    json_data['requests_per_second'] = json_data['runs'].apply(extract_requests)
    cbor_data['requests_per_second'] = cbor_data['runs'].apply(extract_requests)

    json_avg = json_data.groupby('data_type')['requests_per_second'].mean().round().astype(int).reset_index()
    cbor_avg = cbor_data.groupby('data_type')['requests_per_second'].mean().round().astype(int).reset_index()

    combined_avg = pd.merge(json_avg, cbor_avg, on='data_type', suffixes=('_json', '_cbor'))

    combined_avg_sorted = combined_avg.sort_values('data_type', ascending=False)

    combined_avg_sorted = combined_avg_sorted.rename(columns={'data_type': 'Data Type', 'requests_per_second_json': 'JSON', 'requests_per_second_cbor': 'CBOR'})

    return combined_avg_sorted[['Data Type', 'JSON', 'CBOR']].to_markdown(index=False)

json_file_path = 'out/rps-json.csv'
cbor_file_path = 'out/rps-cbor.csv'

markdown_table = process_data(json_file_path, cbor_file_path)
print(markdown_table)
