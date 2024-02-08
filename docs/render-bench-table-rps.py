import pandas as pd

def extract_requests(s):
    return int(s.split()[0])

def process_data(json_path, cbor_path):
    json_data = pd.read_csv(json_path)
    cbor_data = pd.read_csv(cbor_path)
    
    json_data['requests_per_second'] = json_data['runs'].apply(extract_requests)
    cbor_data['requests_per_second'] = cbor_data['runs'].apply(extract_requests)

    json_avg = json_data.groupby(['data_type', 'language'])['requests_per_second'].mean().round().astype(int).reset_index()
    cbor_avg = cbor_data.groupby(['data_type', 'language'])['requests_per_second'].mean().round().astype(int).reset_index()
    
    combined_avg = pd.merge(json_avg, cbor_avg, on=['data_type', 'language'], suffixes=('_json', '_cbor'))
    
    combined_avg_pivoted = combined_avg.pivot(index='data_type', columns='language', values=['requests_per_second_json', 'requests_per_second_cbor'])

    new_columns = []
    for col in combined_avg_pivoted.columns:
        format_type, language = col[0], col[1]
        format_type = 'JSON' if 'json' in format_type else 'CBOR'
        new_columns.append(f"{format_type} ({language.lower()})")
    
    combined_avg_pivoted.columns = new_columns

    final_table = combined_avg_pivoted.reset_index()

    final_table = final_table.rename(columns={'data_type': 'Data Type'})

    desired_order = ['Data Type', 'JSON (go)', 'CBOR (go)', 'JSON (typescript)', 'CBOR (typescript)']
    final_table = final_table[desired_order]

    return final_table.to_markdown(index=False)

json_file_path = 'out/rps-json.csv'
cbor_file_path = 'out/rps-cbor.csv'

markdown_table = process_data(json_file_path, cbor_file_path)
print(markdown_table)
