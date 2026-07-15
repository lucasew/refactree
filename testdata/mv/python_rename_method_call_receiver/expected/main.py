class Box:
    def assist(self):
        return 1

    def stay(self):
        return 2


def make():
    return Box()


def use():
    return make().assist() + Box().assist() + make().stay()
