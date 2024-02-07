import pandas as pd
import numpy as np
import matplotlib.pyplot as plt

def process_data(file_path):
    data = pd.read_csv(file_path)
    data['rps'] = data['runs'].str.extract(r'(\d+)').astype(int)
    average_rps = data.groupby(['data_type', 'language'])['rps'].mean().reset_index()
    sorted_average_rps = average_rps.sort_values(['data_type', 'language']).reset_index(drop=True)
    return sorted_average_rps

def generate_chart(json_data, cbor_data, output_path):
    languages = pd.concat([json_data['language'], cbor_data['language']]).unique()
    data_types = pd.concat([json_data['data_type'], cbor_data['data_type']]).unique()
    
    fig, ax = plt.subplots(figsize=(12, 8))
    bar_width = 0.35
    opacity = 0.8

    colors = plt.cm.tab20(np.linspace(0, 1, len(data_types)))
    json_legend_added = False
    cbor_legend_added = False
    
    for i, language in enumerate(languages):
        json_language_data = json_data[json_data['language'] == language]
        cbor_language_data = cbor_data[cbor_data['language'] == language]
        
        json_bottom = np.zeros(len(languages))
        cbor_bottom = np.zeros(len(languages))
        
        for j, data_type in enumerate(data_types):
            if data_type in json_language_data['data_type'].values:
                json_rps = json_language_data[json_language_data['data_type'] == data_type]['rps'].iloc[0]
                label = f"{data_type} (JSON)" if not json_legend_added else ""
                json_legend_added = True  
                ax.bar(i - bar_width/2, json_rps, bar_width, bottom=json_bottom[i], color=colors[j], label=label, alpha=opacity)
                json_bottom[i] += json_rps
                ax.text(i - bar_width/2, json_bottom[i], f'{data_type} {int(json_rps)}', ha='center', va='bottom')
            
            if data_type in cbor_language_data['data_type'].values:
                cbor_rps = cbor_language_data[cbor_language_data['data_type'] == data_type]['rps'].iloc[0]
                label = f"{data_type} (CBOR)" if not cbor_legend_added else ""
                cbor_legend_added = True  
                ax.bar(i + bar_width/2, cbor_rps, bar_width, bottom=cbor_bottom[i], color=colors[j], hatch='/', label=label, alpha=opacity)
                cbor_bottom[i] += cbor_rps
                ax.text(i + bar_width/2, cbor_bottom[i], f'{data_type} {int(cbor_rps)}', ha='center', va='bottom')

    ax.yaxis.set_visible(False)

    ax.set_xlabel('Language')
    ax.set_ylabel('Average Requests per Second')
    ax.set_title('Average Requests/Second per Data Type for Different Serialization Frameworks and Languages (TCP)')
    ax.set_xticks(np.arange(len(languages)))
    ax.set_xticklabels(languages)
    ax.legend(title='Data Type & Serializer')

    plt.tight_layout()
    plt.savefig(output_path)

if __name__ == '__main__':
    json_file_path = 'out/rps-json.csv'
    cbor_file_path = 'out/rps-cbor.csv'
    output_file_path = 'docs/rps.png'

    json_data = process_data(json_file_path)
    cbor_data = process_data(cbor_file_path)

    generate_chart(json_data, cbor_data, output_file_path)
