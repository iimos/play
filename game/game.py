import pygame
import sys

# Инициализация Pygame
pygame.init()

# Настройки окна
WIDTH, HEIGHT = 800, 600
screen = pygame.display.set_mode((WIDTH, HEIGHT))
pygame.display.set_caption("Простая 2D игра")

# Цвета
WHITE = (255, 255, 255)
RED = (255, 0, 0)

# Настройки персонажа
player_width, player_height = 50, 50
player_x = WIDTH // 2 - player_width // 2
player_y = HEIGHT - player_height - 20
player_speed = 5

# Основной игровой цикл
clock = pygame.time.Clock()
running = True

while running:
    # Обработка событий
    for event in pygame.event.get():
        if event.type == pygame.QUIT:
            running = False
    
    # Обработка нажатий клавиш
    keys = pygame.key.get_pressed()
    if keys[pygame.K_LEFT] and player_x > 0:
        player_x -= player_speed
    if keys[pygame.K_RIGHT] and player_x < WIDTH - player_width:
        player_x += player_speed
    
    # Отрисовка
    screen.fill(WHITE)
    pygame.draw.rect(screen, RED, (player_x, player_y, player_width, player_height))
    
    # Обновление экрана
    pygame.display.flip()
    clock.tick(60)

pygame.quit()
sys.exit()
