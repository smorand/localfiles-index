#!/bin/bash
# Generate test fixture files for localfiles-index E2E tests
# Generates images, PDFs, text files, spreadsheets, and documents

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GENERATED_DIR="$SCRIPT_DIR/generated"

mkdir -p "$GENERATED_DIR"

echo "Generating test fixtures in $GENERATED_DIR..."

# --- Images ---

# Official document image (passport-like) — JPEG
python3 -c "
from PIL import Image, ImageDraw, ImageFont
img = Image.new('RGB', (800, 600), color=(240, 240, 255))
draw = ImageDraw.Draw(img)
draw.rectangle([50, 50, 750, 550], outline='navy', width=3)
draw.text((100, 80), 'REPUBLIQUE FRANCAISE', fill='navy')
draw.text((100, 120), 'PASSEPORT / PASSPORT', fill='navy')
draw.text((100, 180), 'Nom / Surname: MORAND', fill='black')
draw.text((100, 210), 'Prenoms / Given names: Sebastien Pierre', fill='black')
draw.text((100, 240), 'Nationalite / Nationality: Francaise', fill='black')
draw.text((100, 270), 'Date de naissance / Date of birth: 15/03/1985', fill='black')
draw.text((100, 300), 'Lieu de naissance / Place of birth: Paris', fill='black')
draw.text((100, 330), 'Sexe / Sex: M', fill='black')
draw.text((100, 370), 'No. du passeport / Passport No: 12AB34567', fill='navy')
draw.text((100, 400), 'Date de delivrance / Date of issue: 01/06/2020', fill='black')
draw.text((100, 430), 'Date d expiration / Date of expiry: 01/06/2030', fill='black')
draw.text((100, 470), 'Autorite / Authority: Prefecture de Paris', fill='black')
# Photo placeholder
draw.rectangle([550, 150, 700, 350], outline='gray', width=2)
draw.text((580, 240), 'PHOTO', fill='gray')
img.save('$GENERATED_DIR/official_document.jpg', 'JPEG', quality=90)
print('Generated official_document.jpg')
"

# Family photo — JPEG
python3 -c "
from PIL import Image, ImageDraw
img = Image.new('RGB', (1024, 768), color=(135, 206, 235))
draw = ImageDraw.Draw(img)
# Sky gradient effect
for y in range(400):
    r = 135 + int(y * 0.2)
    g = 206 - int(y * 0.1)
    b = 235
    draw.line([(0, y), (1024, y)], fill=(min(r,255), max(g,0), b))
# Green ground
draw.rectangle([0, 400, 1024, 768], fill=(34, 139, 34))
# Trees
for x in [100, 300, 700, 900]:
    draw.rectangle([x-10, 300, x+10, 450], fill=(101, 67, 33))
    draw.ellipse([x-50, 200, x+50, 350], fill=(0, 100, 0))
# People silhouettes
for x in [400, 500, 550]:
    draw.ellipse([x-15, 350, x+15, 380], fill=(60, 60, 60))
    draw.rectangle([x-12, 380, x+12, 450], fill=(60, 60, 60))
# Sun
draw.ellipse([800, 50, 900, 150], fill=(255, 223, 0))
img.save('$GENERATED_DIR/family_photo.jpg', 'JPEG', quality=85)
print('Generated family_photo.jpg')
"

# PNG image (a diagram/screenshot)
python3 -c "
from PIL import Image, ImageDraw
img = Image.new('RGB', (640, 480), color=(255, 255, 255))
draw = ImageDraw.Draw(img)
draw.text((200, 20), 'System Architecture Diagram', fill='black')
# Boxes
draw.rectangle([50, 80, 200, 140], outline='blue', width=2)
draw.text((80, 100), 'Frontend', fill='blue')
draw.rectangle([250, 80, 400, 140], outline='green', width=2)
draw.text((290, 100), 'API', fill='green')
draw.rectangle([450, 80, 600, 140], outline='red', width=2)
draw.text((490, 100), 'Database', fill='red')
# Arrows
draw.line([(200, 110), (250, 110)], fill='black', width=2)
draw.line([(400, 110), (450, 110)], fill='black', width=2)
# Description box
draw.rectangle([50, 200, 600, 400], outline='gray', width=1)
draw.text((60, 210), 'Architecture Description:', fill='black')
draw.text((60, 240), 'The system uses a three-tier architecture:', fill='gray')
draw.text((60, 270), '1. Frontend: React web application', fill='gray')
draw.text((60, 300), '2. API: Go REST service with Fiber', fill='gray')
draw.text((60, 330), '3. Database: PostgreSQL with pgvector', fill='gray')
img.save('$GENERATED_DIR/diagram.png', 'PNG')
print('Generated diagram.png')
"

# Corrupt image (random bytes with .jpg extension)
dd if=/dev/urandom of="$GENERATED_DIR/corrupt_image.jpg" bs=1024 count=5 2>/dev/null
echo "Generated corrupt_image.jpg"

# --- Text files ---

cat > "$GENERATED_DIR/sample_text.txt" << 'TEXTEOF'
The Art of Software Engineering: A Comprehensive Guide

Software engineering is a disciplined approach to the design, development, and maintenance of software systems. It combines principles from computer science, engineering, and project management to create reliable, efficient, and maintainable software products.

Chapter 1: Foundations of Software Development

The foundation of good software development lies in understanding the problem domain thoroughly before writing any code. Requirements gathering, analysis, and specification are critical first steps that determine the success or failure of a project. A well-defined set of requirements serves as the blueprint for the entire development process.

Modern software development methodologies such as Agile, Scrum, and Kanban have transformed how teams collaborate and deliver software. These approaches emphasize iterative development, continuous feedback, and adaptability to change. Teams work in short cycles called sprints, delivering incremental value to stakeholders at regular intervals.

Chapter 2: Design Patterns and Architecture

Design patterns provide reusable solutions to common software design problems. The Gang of Four patterns including Singleton, Factory, Observer, and Strategy patterns remain fundamental to object-oriented design. These patterns promote code reuse, flexibility, and maintainability.

Software architecture defines the high-level structure of a system, including its components, their relationships, and the principles governing their design and evolution. Common architectural styles include microservices, event-driven architecture, and layered architecture. Each style offers different tradeoffs in terms of scalability, maintainability, and complexity.

Chapter 3: Testing and Quality Assurance

Testing is an integral part of software development that ensures the correctness and reliability of the software product. Unit testing verifies individual components in isolation, while integration testing checks the interactions between components. End-to-end testing validates the entire system from the user perspective.

Test-driven development or TDD is a methodology where tests are written before the implementation code. This approach helps developers think about the desired behavior upfront and creates a comprehensive test suite as a byproduct of the development process.
TEXTEOF
echo "Generated sample_text.txt"

# Short text file (French)
cat > "$GENERATED_DIR/document_fr.txt" << 'FREOF'
Rapport Annuel de l'Entreprise XYZ - 2025

Ce rapport presente les resultats financiers et operationnels de l'entreprise XYZ pour l'exercice 2025. L'entreprise a connu une croissance significative de son chiffre d'affaires, atteignant 15 millions d'euros, soit une augmentation de 20% par rapport a l'annee precedente. Les investissements dans la recherche et le developpement ont permis le lancement de trois nouveaux produits innovants sur le marche europeen. La strategie d'expansion internationale a egalement porte ses fruits avec l'ouverture de bureaux a Berlin et Madrid.

Perspectives pour 2026: L'entreprise prevoit de poursuivre sa croissance en investissant dans l'intelligence artificielle et l'automatisation de ses processus de production.
FREOF
echo "Generated document_fr.txt"

# --- Spreadsheets ---

# CSV file
cat > "$GENERATED_DIR/sample.csv" << 'CSVEOF'
Name,Department,Salary,Start Date,Location
Alice Martin,Engineering,85000,2020-03-15,Paris
Bob Dupont,Marketing,72000,2019-07-01,Lyon
Claire Bernard,Engineering,92000,2018-01-10,Paris
David Leroy,Sales,68000,2021-05-20,Marseille
Eve Moreau,Engineering,88000,2020-11-03,Paris
Frank Petit,Marketing,75000,2019-02-14,Lyon
Grace Simon,HR,65000,2022-01-05,Paris
Henri Laurent,Sales,71000,2020-08-22,Bordeaux
CSVEOF
echo "Generated sample.csv"

# XLSX file
python3 -c "
try:
    from openpyxl import Workbook
    wb = Workbook()
    ws = wb.active
    ws.title = 'Sales Data'
    headers = ['Product', 'Q1', 'Q2', 'Q3', 'Q4', 'Total']
    ws.append(headers)
    data = [
        ['Widget A', 15000, 18000, 22000, 25000, 80000],
        ['Widget B', 8000, 9500, 11000, 12000, 40500],
        ['Service X', 30000, 35000, 40000, 45000, 150000],
        ['Service Y', 12000, 14000, 16000, 18000, 60000],
    ]
    for row in data:
        ws.append(row)
    wb.save('$GENERATED_DIR/sample.xlsx')
    print('Generated sample.xlsx')
except ImportError:
    print('SKIP: openpyxl not installed, skipping XLSX generation')
"

# --- PDF (using ImageMagick convert) ---

# Multi-page text PDF with known content
python3 -c "
from PIL import Image, ImageDraw
pages = []
page_texts = [
    ['Page 1: Introduction to Machine Learning',
     '',
     'Machine learning is a subset of artificial intelligence',
     'that focuses on building systems that learn from data.',
     'Instead of being explicitly programmed, these systems',
     'improve their performance through experience.',
     '',
     'Key concepts include supervised learning, unsupervised',
     'learning, and reinforcement learning. Each approach has',
     'specific use cases and applications in modern technology.'],
    ['Page 2: Neural Networks and Deep Learning',
     '',
     'Neural networks are computing systems inspired by',
     'biological neural networks in the human brain.',
     'Deep learning uses multiple layers of neural networks',
     'to progressively extract higher-level features.',
     '',
     'The term UNIQUE_KEYWORD_DEEPLEARNING appears here.',
     'Convolutional neural networks excel at image recognition,',
     'while recurrent networks handle sequential data well.'],
    ['Page 3: Applications and Future Directions',
     '',
     'Machine learning applications span many industries:',
     'healthcare, finance, transportation, and entertainment.',
     'Natural language processing enables chatbots and',
     'translation systems. Computer vision powers autonomous',
     'vehicles and medical imaging analysis.',
     '',
     'Future directions include more efficient training,',
     'better interpretability, and ethical AI frameworks.',
     'Researchers are exploring quantum machine learning.']
]
for i, texts in enumerate(page_texts):
    img = Image.new('RGB', (612, 792), color=(255, 255, 255))
    draw = ImageDraw.Draw(img)
    y = 50
    for text in texts:
        draw.text((50, y), text, fill='black')
        y += 20
    draw.text((500, 750), f'Page {i+1}', fill='gray')
    pages.append(img)
pages[0].save('$GENERATED_DIR/multipage.pdf', 'PDF', save_all=True, append_images=pages[1:], resolution=100.0)
print('Generated multipage.pdf')
"

# --- DOCX file (using python-docx if available) ---

python3 -c "
try:
    from docx import Document
    doc = Document()
    doc.add_heading('Test Document for Conversion', level=1)
    doc.add_paragraph('This is a test DOCX document that will be converted to PDF for indexing.')
    doc.add_paragraph('It contains multiple paragraphs with different content to verify the conversion pipeline works correctly.')
    doc.add_heading('Section 2: Details', level=2)
    doc.add_paragraph('The localfiles-index system should convert this DOCX file to PDF using LibreOffice, then process it through the PDF pipeline. The original file path should be preserved in the database.')
    doc.save('$GENERATED_DIR/sample.docx')
    print('Generated sample.docx')
except ImportError:
    # Fallback: create a minimal DOCX (it is a zip file with XML)
    print('SKIP: python-docx not installed, skipping DOCX generation')
"

# --- Unsupported files ---

dd if=/dev/urandom of="$GENERATED_DIR/archive.zip" bs=1024 count=10 2>/dev/null
echo "Generated archive.zip"

echo ""
echo "Fixture generation complete."
echo "Files in $GENERATED_DIR:"
ls -la "$GENERATED_DIR/"
