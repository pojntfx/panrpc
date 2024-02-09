import pandas as pd
import matplotlib.pyplot as plt
import numpy as np

def load_and_average_throughput_by_language(file_path):
    data = pd.read_csv(file_path)
    data['throughput'] = data['throughput'].str.replace(' MB/s', '').astype(float)

    avg_throughput_by_lang = data.groupby('language')['throughput'].mean().round().reset_index()
    return avg_throughput_by_lang

def plot_throughput_chart(json_data, cbor_data, output_path):
    labels = json_data['language'].unique()
    x = np.arange(len(labels))
    width = 0.35

    fig, ax = plt.subplots(figsize=(10, 6))
    rects1 = ax.bar(x - width/2, json_data['throughput'], width, label='JSON', color='orange')
    rects2 = ax.bar(x + width/2, cbor_data['throughput'], width, label='CBOR', color='royalblue')

    ax.set_ylabel('Throughput (MB/s)')
    ax.set_title('Average Throughput for Different Serialization Frameworks and Languages (TCP)')
    ax.set_xticks(x)
    ax.set_xticklabels(labels)
    ax.legend()

    def autolabel(rects):
        """Attach a text label above each bar in *rects*, displaying its height."""
        for rect in rects:
            height = rect.get_height()
            ax.annotate('{}'.format(int(height)),  # Convert to integer for display
                        xy=(rect.get_x() + rect.get_width() / 2, height),
                        xytext=(0, 3),  # 3 points vertical offset
                        textcoords="offset points",
                        ha='center', va='bottom')

    autolabel(rects1)
    autolabel(rects2)

    fig.tight_layout()

    plt.savefig(output_path)

if __name__ == '__main__':
    json_file_path = 'out/throughput-json.csv'
    cbor_file_path = 'out/throughput-cbor.csv'
    output_file_path = 'docs/throughput.png'

    json_data = load_and_average_throughput_by_language(json_file_path)
    cbor_data = load_and_average_throughput_by_language(cbor_file_path)

    json_data = json_data.sort_values(by='language').reset_index(drop=True)
    cbor_data = cbor_data.sort_values(by='language').reset_index(drop=True)

    plot_throughput_chart(json_data, cbor_data, output_file_path)
