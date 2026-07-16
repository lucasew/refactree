class Box:
    def helper(self):
        return 1

    def stay(self):
        return 2


def make():
    return Box()


def use():
    return make().helper() + Box().helper() + make().stay()
