import pandas as pd
import matplotlib.pyplot as plt

def load_and_average_throughput(file_path):
    data = pd.read_csv(file_path)
    data['throughput'] = data['throughput'].str.replace(' MB/s', '').astype(int)

    return data['throughput'].mean()

def plot_throughput_chart(json_avg, cbor_avg, output_path):
    labels = ['JSON', 'CBOR']
    throughputs = [json_avg, cbor_avg]

    plt.figure(figsize=(10, 6))
    plt.bar(labels, throughputs, color=['green', 'blue'])
    plt.title('Average Throughput for Different Serialization Frameworks (TCP)')
    plt.ylabel('Throughput (MB/s)')

    plt.savefig(output_path)

if __name__ == '__main__':
    json_file_path = 'out/throughput-json.csv'
    cbor_file_path = 'out/throughput-cbor.csv'
    output_file_path = 'docs/throughput.png'

    json_avg_throughput = load_and_average_throughput(json_file_path)
    cbor_avg_throughput = load_and_average_throughput(cbor_file_path)

    plot_throughput_chart(json_avg_throughput, cbor_avg_throughput, output_file_path)
