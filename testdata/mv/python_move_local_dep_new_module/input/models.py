class Config:
    def __init__(self, name):
        self.name = name

def create_config(name):
    return Config(name)
