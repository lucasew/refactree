class Box:
    @property
    def assist(self):
        return 1

    @assist.setter
    def assist(self, v):
        pass

    def stay(self):
        return 2


def use(b: Box):
    return b.assist + b.stay()
