class Box:
    def assist(self):
        return 1

    def stay(self):
        return 2


def use():
    return Box().assist() + Box().stay()


def typed(b: Box):
    return b.assist()
