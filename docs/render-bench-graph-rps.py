import pandas as pd
import numpy as np
import matplotlib.pyplot as plt

def process_data(file_path):
    data = pd.read_csv(file_path)
    data['rps'] = data['runs'].str.extract(r'(\d+)').astype(int)
    average_rps = data.groupby('data_type')['rps'].mean().reset_index()
    sorted_average_rps = average_rps.sort_values('data_type').reset_index(drop=True)
    return sorted_average_rps

def generate_chart(json_data, cbor_data, output_path):
    common_data_types = json_data[json_data['data_type'].isin(cbor_data['data_type'])]['data_type']
    filtered_json_rps = json_data[json_data['data_type'].isin(common_data_types)]
    filtered_cbor_rps = cbor_data[cbor_data['data_type'].isin(common_data_types)]

    fig, ax = plt.subplots(figsize=(10, 8))
    json_bottoms = filtered_json_rps['rps'].cumsum() - filtered_json_rps['rps']
    cbor_bottoms = filtered_cbor_rps['rps'].cumsum() - filtered_cbor_rps['rps']
    sorted_colors = plt.cm.tab20(np.linspace(0, 1, len(common_data_types)))

    # JSON bar
    for (data_type, rps, bottom, color) in zip(filtered_json_rps['data_type'], filtered_json_rps['rps'], json_bottoms, sorted_colors):
        ax.bar('JSON', rps, bottom=bottom, color=color)
        rps_formatted = f"{int(round(rps)):,}"
        ax.text('JSON', bottom + rps/2, f'{data_type}\n{rps_formatted}', ha='center', va='center', color='white')

    # CBOR bar
    for (data_type, rps, bottom, color) in zip(filtered_cbor_rps['data_type'], filtered_cbor_rps['rps'], cbor_bottoms, sorted_colors):
        ax.bar('CBOR', rps, bottom=bottom, color=color)
        rps_formatted = f"{int(round(rps)):,}"
        ax.text('CBOR', bottom + rps/2, f'{data_type}\n{rps_formatted}', ha='center', va='center', color='white')

    ax.set_title('Average Requests/Second per Data Type for Different Serialization Frameworks (TCP)')
    ax.yaxis.set_visible(False)
    plt.tight_layout()

    plt.savefig(output_path)

if __name__ == '__main__':
    json_file_path = 'out/rps-json.csv'
    cbor_file_path = 'out/rps-cbor.csv'
    output_file_path = 'docs/rps.png'

    json_data = process_data(json_file_path)
    cbor_data = process_data(cbor_file_path)

    generate_chart(json_data, cbor_data, output_file_path)
